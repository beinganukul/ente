//go:build !js

package database

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Uses the official connector's HTTP response decoding, including its error
// wrapping and stream state, rather than simulating only a driver error string.
type missingStreamTransport struct {
	mu              sync.Mutex
	failure         string
	lookups         int
	streams         int
	batons          []string
	noBaton         bool
	closeAfterQuery bool
}

func (t *missingStreamTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	var pipeline struct {
		Baton    string `json:"baton"`
		Requests []struct {
			Type string `json:"type"`
			Stmt struct {
				SQL string `json:"sql"`
			} `json:"stmt"`
		} `json:"requests"`
	}
	if err := json.NewDecoder(request.Body).Decode(&pipeline); err != nil {
		return nil, err
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	query := pipeline.Requests[0].Stmt.SQL
	baton := pipeline.Baton
	if query == "PRAGMA foreign_keys=ON" {
		t.streams++
		baton = fmt.Sprintf("stream-%d", t.streams)
		if t.noBaton {
			baton = ""
		}
	}
	if strings.Contains(query, "users") {
		t.lookups++
		t.batons = append(t.batons, pipeline.Baton)
		if t.closeAfterQuery {
			// Keep the bounded pool busy while the other requests queue.
			time.Sleep(time.Millisecond)
			baton = ""
		}
		if t.lookups == 1 {
			switch t.failure {
			case "missing":
				return pipelineHTTPResponse(http.StatusNotFound, `{"error":"stream not found: opaque-stream-id"}`), nil
			case "unrelated":
				return pipelineHTTPResponse(http.StatusNotFound, `{"error":"database not found"}`), nil
			case "ambiguous":
				return nil, errors.New("ambiguous network interruption")
			case "server":
				return pipelineHTTPResponse(http.StatusInternalServerError, `{"error":"stream not found: opaque-stream-id"}`), nil
			case "gateway":
				return pipelineHTTPResponse(http.StatusBadGateway, `{"error":"upstream unavailable"}`), nil
			}
		}
	}
	result := `{"cols":[],"rows":[],"affected_row_count":1,"last_insert_rowid":"1"}`
	if strings.HasPrefix(query, "SELECT") {
		result = `{"cols":[{"name":"user_id","decltype":"INTEGER"}],"rows":[[{"type":"integer","value":"42"}]],"affected_row_count":0}`
	}
	body := fmt.Sprintf(`{"baton":%q,"results":[{"type":"ok","response":{"type":"execute","result":%s}}]}`, baton, result)
	return pipelineHTTPResponse(http.StatusOK, body), nil
}

func pipelineHTTPResponse(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}
}

func TestMissingHranaStreamRecovery(t *testing.T) {
	for _, operation := range []string{"query", "exec"} {
		for _, failure := range []string{"missing", "unrelated", "ambiguous", "server"} {
			t.Run(operation+"/"+failure, func(t *testing.T) {
				transport := &missingStreamTransport{failure: failure}
				previous := http.DefaultClient
				http.DefaultClient = &http.Client{Transport: transport}
				defer func() { http.DefaultClient = previous }()
				db, err := Open(context.Background(), Config{Provider: Turso, TursoURL: "http://127.0.0.1:1"})
				require.NoError(t, err)
				defer db.Close()
				initialStreams := transport.streams
				var userID int64
				if operation == "query" {
					err = db.QueryRow(`SELECT user_id FROM users WHERE email_hash = $1`, "fixture-hash").Scan(&userID)
				} else {
					_, err = db.Exec(`UPDATE users SET name = 'fixture'`)
				}
				if failure == "missing" {
					require.NoError(t, err)
					if operation == "query" {
						require.Equal(t, int64(42), userID)
					}
					require.Equal(t, 2, transport.lookups)
					require.Equal(t, []string{fmt.Sprintf("stream-%d", initialStreams+1), fmt.Sprintf("stream-%d", initialStreams+2)}, transport.batons)
				} else {
					require.Error(t, err)
					require.NotErrorIs(t, err, driver.ErrBadConn)
					require.Equal(t, 1, transport.lookups, "unrecognized failures must not replay")
				}
			})
		}
	}
}

func TestMissingHranaStreamTransactionBodyDoesNotReplay(t *testing.T) {
	for _, operation := range []string{"query", "exec"} {
		t.Run(operation, func(t *testing.T) {
			transport := &missingStreamTransport{failure: "missing"}
			previous := http.DefaultClient
			http.DefaultClient = &http.Client{Transport: transport}
			defer func() { http.DefaultClient = previous }()
			db, err := Open(context.Background(), Config{Provider: Turso, TursoURL: "http://127.0.0.1:1"})
			require.NoError(t, err)
			defer db.Close()
			tx, err := db.Begin()
			require.NoError(t, err)
			defer tx.Rollback()
			if operation == "query" {
				var userID int64
				err = tx.QueryRow(`SELECT user_id FROM users WHERE email_hash = $1`, "fixture-hash").Scan(&userID)
			} else {
				_, err = tx.Exec(`UPDATE users SET name = 'fixture'`)
			}
			require.ErrorIs(t, err, driver.ErrBadConn)
			require.Equal(t, 1, transport.lookups, "transaction bodies must not replay")
		})
	}
}

func TestClosedHranaStreamRecovery(t *testing.T) {
	for _, operation := range []string{"query", "exec"} {
		for _, transaction := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/transaction=%t", operation, transaction), func(t *testing.T) {
				transport := &missingStreamTransport{failure: "gateway"}
				previous := http.DefaultClient
				http.DefaultClient = &http.Client{Transport: transport}
				defer func() { http.DefaultClient = previous }()
				db, err := Open(context.Background(), Config{Provider: Turso, TursoURL: "http://127.0.0.1:1"})
				require.NoError(t, err)
				defer db.Close()
				initialStreams := transport.streams
				exec := db.Exec
				query := db.QueryRow
				if transaction {
					tx, err := db.Begin()
					require.NoError(t, err)
					defer tx.Rollback()
					exec, query = tx.Exec, tx.QueryRow
				}
				perform := func() error {
					if operation == "query" {
						var userID int64
						return query(`SELECT user_id FROM users WHERE email_hash = $1`, "fixture-hash").Scan(&userID)
					}
					_, err := exec(`UPDATE users SET name = 'fixture'`)
					return err
				}
				err = perform()
				require.ErrorContains(t, err, "error code 502")
				require.NotErrorIs(t, err, driver.ErrBadConn)
				require.Equal(t, 1, transport.lookups, "uncertain HTTP failure must not replay")
				err = perform()
				if transaction {
					require.ErrorIs(t, err, driver.ErrBadConn)
					require.Equal(t, 1, transport.lookups, "transaction bodies must not replay")
				} else {
					require.NoError(t, err)
					require.Equal(t, 2, transport.lookups)
					require.Equal(t, []string{fmt.Sprintf("stream-%d", initialStreams+1), fmt.Sprintf("stream-%d", initialStreams+2)}, transport.batons)
				}
			})
		}
	}
}

func TestMissingHranaBatonExhaustsSafeConnectionRetries(t *testing.T) {
	for _, operation := range []string{"query", "exec"} {
		t.Run(operation, func(t *testing.T) {
			transport := &missingStreamTransport{noBaton: true}
			previous := http.DefaultClient
			http.DefaultClient = &http.Client{Transport: transport}
			defer func() { http.DefaultClient = previous }()
			db, err := Open(context.Background(), Config{Provider: Turso, TursoURL: "http://127.0.0.1:1"})
			require.NoError(t, err)
			defer db.Close()
			initialStreams := transport.streams
			if operation == "query" {
				var userID int64
				err = db.QueryRow(`SELECT user_id FROM users WHERE email_hash = $1`, "fixture-hash").Scan(&userID)
			} else {
				_, err = db.Exec(`UPDATE users SET name = 'fixture'`)
			}
			require.ErrorIs(t, err, driver.ErrBadConn)
			require.ErrorContains(t, err, "stream is closed")
			require.Equal(t, initialStreams+3, transport.streams, "database/sql exhausted three fresh streams")
			require.Zero(t, transport.lookups, "SQL must never reach a closed stream")
		})
	}
}

func TestHranaClosingResponsesUnderConcurrentQueries(t *testing.T) {
	for _, maxIdle := range []int{2, 0} {
		t.Run(fmt.Sprintf("maxIdle=%d", maxIdle), func(t *testing.T) {
			transport := &missingStreamTransport{closeAfterQuery: true}
			previous := http.DefaultClient
			http.DefaultClient = &http.Client{Transport: transport}
			defer func() { http.DefaultClient = previous }()
			db, err := Open(context.Background(), Config{Provider: Turso, TursoURL: "http://127.0.0.1:1"})
			require.NoError(t, err)
			defer db.Close()
			db.SetMaxOpenConns(2)
			db.SetMaxIdleConns(maxIdle)
			for round := 0; round < 10; round++ {
				start := make(chan struct{})
				results := make(chan error, 4)
				for request := 0; request < 4; request++ {
					go func() {
						<-start
						var userID int64
						results <- db.QueryRow(`SELECT user_id FROM users`).Scan(&userID)
					}()
				}
				close(start)
				var failures []error
				for request := 0; request < 4; request++ {
					if err := <-results; err != nil {
						failures = append(failures, err)
					}
				}
				require.Empty(t, failures, "round %d", round)
			}
			require.Equal(t, 40, transport.lookups)
		})
	}
}

func TestTursoTransactionRetainsStreamUntilBoundary(t *testing.T) {
	for _, commit := range []bool{false, true} {
		t.Run(fmt.Sprintf("commit=%t", commit), func(t *testing.T) {
			transport := &missingStreamTransport{}
			previous := http.DefaultClient
			http.DefaultClient = &http.Client{Transport: transport}
			defer func() { http.DefaultClient = previous }()
			db, err := Open(context.Background(), Config{Provider: Turso, TursoURL: "http://127.0.0.1:1"})
			require.NoError(t, err)
			defer db.Close()
			initialStreams := transport.streams
			tx, err := db.Begin()
			require.NoError(t, err)
			defer tx.Rollback()
			for i := 0; i < 3; i++ {
				var userID int64
				require.NoError(t, tx.QueryRow(`SELECT user_id FROM users`).Scan(&userID))
				require.Equal(t, int64(42), userID)
			}
			require.Equal(t, initialStreams+1, transport.streams)
			for _, baton := range transport.batons {
				require.Equal(t, fmt.Sprintf("stream-%d", initialStreams+1), baton)
			}
			if commit {
				require.NoError(t, tx.Commit())
			} else {
				require.NoError(t, tx.Rollback())
			}
			_, err = db.Exec(`UPDATE users SET name = 'fixture'`)
			require.NoError(t, err)
			require.Equal(t, initialStreams+2, transport.streams, "completed transaction must retire its stream")
		})
	}
}

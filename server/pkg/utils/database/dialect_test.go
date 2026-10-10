package database

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"testing"

	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestNumberedParameters(t *testing.T) {
	require.Equal(t, `SELECT ?2, ?1, ?2, '$1', "$2" -- $3
/* $4 */ ?1`, numberedParameters(`SELECT $2, $1, $2, '$1', "$2" -- $3
/* $4 */ $1`))
}

type expiredStreamConn struct {
	driver.Conn
	updates int
}

func (c *expiredStreamConn) Close() error { return nil }
func (c *expiredStreamConn) ExecContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Result, error) {
	if query == "BEGIN IMMEDIATE" && c.updates == -1 {
		c.updates = 0
		return driver.RowsAffected(0), nil
	}
	if query == "UPDATE users SET name='test'" {
		c.updates++
	}
	return nil, errors.New("failed to execute SQL:\nerror code 400: ")
}

func TestExpiredStreamBeforeBeginUsesFreshConnection(t *testing.T) {
	conn := &tursoConn{Conn: &expiredStreamConn{}}
	_, err := conn.BeginTx(context.Background(), driver.TxOptions{})
	require.ErrorIs(t, err, driver.ErrBadConn)
	require.False(t, conn.IsValid())
}

type testStreamConnector struct{ conn driver.Conn }

func (c testStreamConnector) Driver() driver.Driver                        { return tursoDriver{} }
func (c testStreamConnector) Connect(context.Context) (driver.Conn, error) { return c.conn, nil }

func TestExpiredStreamDuringTransactionIsNotReplayed(t *testing.T) {
	inner := &expiredStreamConn{updates: -1}
	conn := &tursoConn{Conn: inner}
	db := sql.OpenDB(testStreamConnector{conn: conn})
	defer db.Close()
	tx, err := db.Begin()
	require.NoError(t, err)
	defer tx.Rollback()
	_, err = tx.Exec("UPDATE users SET name='test'")
	require.ErrorIs(t, err, driver.ErrBadConn)
	require.Equal(t, 1, inner.updates, "transaction bodies must not retry")
	require.False(t, conn.IsValid())
}

func TestUniqueConstraintIdentity(t *testing.T) {
	for _, err := range []error{
		&pq.Error{Code: "23505", Constraint: "object_keys_object_key_key"},
		errors.New("SQLite error: UNIQUE constraint failed: object_keys.object_key"),
	} {
		require.True(t, IsUniqueViolationFor(err, "object_keys_object_key_key", "object_keys.object_key"))
		require.False(t, IsUniqueViolationFor(err, "object_keys_pkey", "object_keys.file_id, object_keys.o_type"))
	}
	require.False(t, IsUniqueViolationFor(errors.New("FOREIGN KEY constraint failed"), "object_keys_object_key_key", "object_keys.object_key"))
}

type failedTransactionConn struct {
	driver.Conn
	failSQL string
	closed  bool
	calls   []string
}

func (c *failedTransactionConn) Close() error { c.closed = true; return nil }
func (c *failedTransactionConn) ExecContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Result, error) {
	c.calls = append(c.calls, query)
	if query == c.failSQL {
		return nil, errors.New("ambiguous network interruption")
	}
	return driver.RowsAffected(1), nil
}

type transactionPoolConnector struct {
	connections []*tursoConn
	opened      int
}

func (c *transactionPoolConnector) Driver() driver.Driver { return tursoDriver{} }
func (c *transactionPoolConnector) Connect(context.Context) (driver.Conn, error) {
	conn := c.connections[c.opened]
	c.opened++
	return conn, nil
}

func TestFailedTransactionBoundaryDiscardsConnection(t *testing.T) {
	for _, boundary := range []string{"BEGIN IMMEDIATE", "COMMIT", "ROLLBACK"} {
		t.Run(boundary, func(t *testing.T) {
			first := &failedTransactionConn{failSQL: boundary}
			second := &failedTransactionConn{}
			connector := &transactionPoolConnector{connections: []*tursoConn{{Conn: first}, {Conn: second}}}
			db := sql.OpenDB(connector)
			defer db.Close()
			db.SetMaxOpenConns(1)
			tx, err := db.Begin()
			if boundary == "BEGIN IMMEDIATE" {
				require.ErrorContains(t, err, "ambiguous network interruption")
			} else {
				require.NoError(t, err)
				_, err = tx.Exec("INSERT INTO first VALUES(1)")
				require.NoError(t, err)
				if boundary == "COMMIT" {
					err = tx.Commit()
				} else {
					err = tx.Rollback()
				}
				require.ErrorContains(t, err, "ambiguous network interruption")
			}
			_, err = db.Exec("INSERT INTO later VALUES(2)")
			require.NoError(t, err)
			require.True(t, first.closed)
			require.Equal(t, 2, connector.opened)
			require.Equal(t, []string{"INSERT INTO later VALUES(2)"}, second.calls)
			var attempts int
			for _, query := range first.calls {
				if query == boundary {
					attempts++
				}
			}
			require.Equal(t, 1, attempts, "ambiguous transaction boundary must not replay")
		})
	}
}
func TestJSONArrayScanner(t *testing.T) {
	var result []string
	a := &jsonArray{value: &result}
	require.NoError(t, a.Scan(`["one","two"]`))
	require.Equal(t, []string{"one", "two"}, result)
	v, err := a.Value()
	require.NoError(t, err)
	require.Equal(t, `["one","two"]`, v)
	require.NoError(t, a.Scan(nil))
	require.Nil(t, result)
}

func TestJSONArrayScannerRejectsNullPrimitiveElements(t *testing.T) {
	for _, target := range []any{&[]string{"original"}, &[]int64{42}, &[]bool{true}} {
		err := (&jsonArray{value: target}).Scan(`[null]`)
		require.ErrorContains(t, err, "NULL array element")
	}
	strings := []string{"original"}
	require.Error(t, (&jsonArray{value: &strings}).Scan(`["b5",null]`))
	require.Equal(t, []string{"original"}, strings, "invalid storage buckets must not replace the destination")
	require.NoError(t, (&jsonArray{value: &strings}).Scan(`[]`))
	require.NotNil(t, strings, "empty arrays differ from SQL NULL")
	var integers []int64
	require.NoError(t, (&jsonArray{value: &integers}).Scan(`[42]`))
	require.Equal(t, []int64{42}, integers)
	var nullable []*string
	require.NoError(t, (&jsonArray{value: &nullable}).Scan(`[null,"b5"]`))
	require.Nil(t, nullable[0])
	require.Equal(t, "b5", *nullable[1])
}

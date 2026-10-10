//go:build js && wasm

package main

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"syscall/js"
	"time"

	"github.com/ente/museum/pkg/utils/database"
	"github.com/ente/museum/pkg/utils/workerjobs"
	"github.com/gin-gonic/gin"
	"github.com/lib/pq"
	"github.com/robfig/cron/v3"
	log "github.com/sirupsen/logrus"
	workers "github.com/syumai/workers-go"
	"github.com/syumai/workers-go/cloudflare/sockets"
	"github.com/syumai/workers-go/exp/cloudflare/durableobjects"
)

var workerState *durableobjects.DurableObjectState
var workerCron *cron.Cron
var workerMigrationVersion string
var workerTursoMigrationVersion string

type museumObject struct{ server *gin.Engine }

func main() {
	durableobjects.Register("Museum", func(state *durableobjects.DurableObjectState, env js.Value) (object durableobjects.Object, err error) {
		defer func() {
			if failure := recover(); failure != nil {
				err = fmt.Errorf("Museum startup: %v", failure)
			}
			if err != nil {
				// A retry needs fresh Go globals, including database pools and collectors.
				_ = state.Abort("Museum startup failed", durableobjects.DurableObjectAbortOptions{RetryAlarm: true})
			}
		}()
		workerState = state
		workerjobs.BackgroundState = state
		object = &museumObject{server: newServer()}
		workerCron = workerjobs.Scheduler
		if len(workerCron.Entries()) > 0 {
			err = initializeAlarms(state)
		}
		return object, err
	})
	workers.Serve(nil)
}

func (m *museumObject) ServeHTTP(w http.ResponseWriter, r *http.Request) { m.server.ServeHTTP(w, r) }

func initializeAlarms(state *durableobjects.DurableObjectState) error {
	var next map[string]time.Time
	if _, err := state.Storage().GetJSON("museum_cron", &next); err != nil {
		return err
	}
	if next == nil {
		next = make(map[string]time.Time)
	}
	now := time.Now()
	for _, entry := range workerCron.Entries() {
		key := workerjobs.Keys[entry.ID]
		if _, ok := next[key]; !ok {
			next[key] = entry.Schedule.Next(now)
			if workerjobs.Immediate[entry.ID] {
				next[key] = now
			}
		}
	}
	if err := state.Storage().PutJSON("museum_cron", next); err != nil {
		return err
	}
	return state.Storage().SetAlarm(now.Add(time.Minute), durableobjects.DurableObjectSetAlarmOptions{})
}

func (m *museumObject) Alarm(ctx context.Context, _ *durableobjects.AlarmInvocationInfo) error {
	storage := workerState.Storage()
	if err := storage.SetAlarm(time.Now().Add(time.Minute), durableobjects.DurableObjectSetAlarmOptions{}); err != nil {
		return err
	}
	var next map[string]time.Time
	if _, err := storage.GetJSON("museum_cron", &next); err != nil {
		return err
	}
	if next == nil {
		return initializeAlarms(workerState)
	}
	entries := workerCron.Entries()
	entry, deadline := workerjobs.Next(entries, next)
	if entry.ID != 0 && !deadline.After(time.Now()) {
		// Each job gets a separate invocation's external-subrequest budget.
		key := workerjobs.Keys[entry.ID]
		if err := runAlarmJob(entry.Job); err != nil {
			log.WithError(err).WithField("job", key).Error("Worker job failed")
		}
		next[key] = entry.Schedule.Next(time.Now())
		if err := storage.PutJSON("museum_cron", next); err != nil {
			return err
		}
	}
	_, deadline = workerjobs.Next(entries, next)
	if deadline.IsZero() {
		return nil
	}
	if earliest := time.Now().Add(time.Second); deadline.Before(earliest) {
		deadline = earliest
	}
	return storage.SetAlarm(deadline, durableobjects.DurableObjectSetAlarmOptions{})
}

func runAlarmJob(job cron.Job) (err error) {
	defer func() {
		if failure := recover(); failure != nil {
			err = fmt.Errorf("%v", failure)
		}
	}()
	job.Run()
	return nil
}

func serverHostName() (string, error) { return workerState.ID().String() }
func startServerCron(c *cron.Cron)    { workerCron = c }

type workerDialer struct{}

func (workerDialer) Dial(_, address string) (net.Conn, error) {
	return sockets.Connect(context.Background(), address, nil)
}
func (workerDialer) DialTimeout(_, address string, timeout time.Duration) (net.Conn, error) {
	connection, err := sockets.Connect(context.Background(), address, nil)
	if err == nil {
		err = connection.SetDeadline(time.Now().Add(timeout))
	}
	return connection, err
}
func (workerDialer) DialContext(ctx context.Context, _, address string) (net.Conn, error) {
	// SQL pool connections outlive the request that opens them.
	return sockets.Connect(context.WithoutCancel(ctx), address, nil)
}

func openServerDatabase(ctx context.Context, cfg database.Config) (*sql.DB, error) {
	provider, err := cfg.EffectiveProvider()
	if err != nil {
		return nil, err
	}
	if provider != database.Postgres {
		return database.Open(ctx, cfg)
	}
	connector, err := pq.NewConnector(cfg.PostgresDSN)
	if err != nil {
		return nil, err
	}
	connector.Dialer(workerDialer{})
	db := sql.OpenDB(connector)
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, err
	}
	db.SetMaxOpenConns(4)
	return db, nil
}

func migrateServerDatabase(ctx context.Context, cfg database.Config) error {
	provider, err := cfg.EffectiveProvider()
	if err != nil {
		return err
	}
	db, err := openServerDatabase(ctx, cfg)
	if err != nil {
		return err
	}
	defer db.Close()
	var version int
	var dirty bool
	if err := db.QueryRowContext(ctx, "SELECT version, dirty FROM schema_migrations").Scan(&version, &dirty); err != nil {
		return fmt.Errorf("run cmd/database migrations before deploying Museum: %w", err)
	}
	if dirty {
		return fmt.Errorf("database migration %d is dirty", version)
	}
	versionString := workerMigrationVersion
	if provider == database.Turso {
		versionString = workerTursoMigrationVersion
	}
	expected, err := strconv.Atoi(versionString)
	if err != nil {
		return fmt.Errorf("build the Worker using workers/build.mjs to embed its schema version")
	}
	if version < expected {
		return fmt.Errorf("database schema %d is behind Worker schema %d; run cmd/database migrations", version, expected)
	}
	return nil
}

//go:build !js

package main

import (
	"context"
	"database/sql"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/ente/museum/pkg/utils/database"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/robfig/cron/v3"
)

func main() {
	server := newServer()
	defer closeServer()
	http.Handle("/metrics", promhttp.Handler())
	go http.ListenAndServe(":2112", nil)
	go runServer(os.Getenv("ENVIRONMENT"), server)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
}

func serverHostName() (string, error) { return os.Hostname() }
func openServerDatabase(ctx context.Context, cfg database.Config) (*sql.DB, error) {
	return database.Open(ctx, cfg)
}
func migrateServerDatabase(ctx context.Context, cfg database.Config) error {
	return database.Migrate(ctx, cfg, "migrations")
}
func startServerCron(c *cron.Cron) { c.Start() }

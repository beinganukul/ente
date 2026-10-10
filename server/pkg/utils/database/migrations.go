//go:build !js

package database

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	migratedb "github.com/golang-migrate/migrate/v4/database"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	"github.com/golang-migrate/migrate/v4/source/file"
)

func Migrate(ctx context.Context, cfg Config, root string) error {
	return migrateWithTable(ctx, cfg, root, "")
}

func migrateWithTable(ctx context.Context, cfg Config, root, table string) error {
	provider, err := cfg.EffectiveProvider()
	if err != nil {
		return err
	}
	dialect := "postgres"
	if provider == Turso {
		dialect = "sqlite"
	}
	directory, err := filepath.Abs(filepath.Join(root, dialect))
	if err != nil {
		return err
	}
	files, err := filepath.Glob(filepath.Join(directory, "*.up.sql"))
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no %s migrations in %s", dialect, directory)
	}
	// The migration adapters own their connections; never give them an application pool.
	db, err := Open(ctx, cfg)
	if err != nil {
		return err
	}
	defer db.Close()
	if provider == Turso {
		leaseDB, leaseErr := Open(ctx, cfg)
		if leaseErr != nil {
			return leaseErr
		}
		defer leaseDB.Close()
		release, leaseErr := acquireMigrationLease(ctx, leaseDB)
		if leaseErr != nil {
			return fmt.Errorf("lock SQLite migrations: %w", leaseErr)
		}
		defer release()
	}

	var driver migratedb.Driver
	if provider == Postgres {
		driver, err = postgres.WithInstance(db, &postgres.Config{MigrationsTable: table})
	} else {
		// Keep the adapter's explicit transaction; multi-statement libSQL Exec alone is not atomic.
		driver, err = sqlite3.WithInstance(db, &sqlite3.Config{MigrationsTable: table})
	}
	if err != nil {
		return fmt.Errorf("create %s migration adapter: %w", dialect, err)
	}
	defer driver.Close()
	source, err := (&file.File{}).Open("file://" + filepath.ToSlash(directory))
	if err != nil {
		return fmt.Errorf("load %s migrations: %w", dialect, err)
	}
	defer source.Close()
	m, err := migrate.NewWithInstance("file", source, dialect, driver)
	if err != nil {
		return fmt.Errorf("load %s migrations: %w", dialect, err)
	}
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("apply %s migrations: %w", dialect, err)
	}
	return nil
}

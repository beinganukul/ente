//go:build !js

package database

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
)

// A database lease protects migration runners in separate Museum processes.
// It is outside the DDL transaction, so remote transaction deadlines still apply
// to each migration file. A crashed runner's lease expires after ten minutes.
func acquireMigrationLease(ctx context.Context, db *sql.DB) (func(), error) {
	_, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS museum_migration_lock(id INTEGER PRIMARY KEY CHECK(id=1),owner TEXT NOT NULL,expires INTEGER NOT NULL)`)
	if err != nil {
		return nil, err
	}
	owner := uuid.NewString()
	result, err := db.ExecContext(ctx, `INSERT INTO museum_migration_lock(id,owner,expires) VALUES(1,$1,unixepoch()+600) ON CONFLICT(id) DO UPDATE SET owner=excluded.owner,expires=excluded.expires WHERE museum_migration_lock.expires<unixepoch()`, owner)
	if err != nil {
		return nil, err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if count != 1 {
		return nil, errors.New("another process is applying SQLite migrations; retry after it finishes or its ten-minute lease expires")
	}
	return func() { _, _ = db.Exec(`DELETE FROM museum_migration_lock WHERE id=1 AND owner=$1`, owner) }, nil
}

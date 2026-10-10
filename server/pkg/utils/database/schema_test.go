//go:build !js

package database

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMuseumSchemaParity(t *testing.T) {
	endpoint := os.Getenv("ENTE_TEST_TURSO_URL")
	dsn := os.Getenv("ENTE_TEST_POSTGRES_DSN")
	if endpoint == "" || dsn == "" {
		t.Skip("requires disposable libSQL and PostgreSQL databases")
	}
	ctx := context.Background()
	pg, err := Open(ctx, Config{PostgresDSN: dsn})
	require.NoError(t, err)
	defer pg.Close()
	var pgName string
	require.NoError(t, pg.QueryRow(`SELECT current_database()`).Scan(&pgName))
	require.True(t, strings.HasPrefix(pgName, "ente_test_"))
	sqlite, err := Open(ctx, Config{Provider: Turso, TursoURL: endpoint})
	require.NoError(t, err)
	defer sqlite.Close()
	root := filepath.Join("..", "..", "..", "migrations")
	require.NoError(t, Migrate(ctx, Config{PostgresDSN: dsn}, root))
	require.NoError(t, Migrate(ctx, Config{Provider: Turso, TursoURL: endpoint}, root))
	pgTables := tableColumns(t, pg, `SELECT table_name,column_name FROM information_schema.columns WHERE table_schema='public' AND table_name!='schema_migrations' ORDER BY table_name,ordinal_position`)
	sqliteTables := tableColumns(t, sqlite, `SELECT m.name,p.name FROM sqlite_master m JOIN pragma_table_xinfo(m.name) p WHERE m.type='table' AND m.name NOT LIKE 'sqlite_%' AND m.name NOT IN ('schema_migrations','museum_migration_lock','db_probe_migrations') ORDER BY m.name,p.cid`)
	require.Equal(t, pgTables, sqliteTables, "both migration tracks must expose identical Museum tables and columns, including generated columns")
	require.Len(t, pgTables, 83)
	rows, err := sqlite.Query(`PRAGMA foreign_key_check`)
	require.NoError(t, err)
	require.False(t, rows.Next(), "SQLite foreign key consistency")
	require.NoError(t, rows.Close())
	var version int
	var dirty bool
	require.NoError(t, sqlite.QueryRow(`SELECT version,dirty FROM schema_migrations`).Scan(&version, &dirty))
	require.Equal(t, 153, version)
	require.False(t, dirty)
	// Every connection must enforce foreign keys, even after the pool expands.
	conn, err := sqlite.Conn(ctx)
	require.NoError(t, err)
	defer conn.Close()
	var fk int
	require.NoError(t, conn.QueryRowContext(ctx, `PRAGMA foreign_keys`).Scan(&fk))
	require.Equal(t, 1, fk)
	require.NoError(t, sqlite.QueryRow(`PRAGMA foreign_keys`).Scan(&fk))
	require.Equal(t, 1, fk)
}

func TestTursoMigrationLeaseAcrossPools(t *testing.T) {
	endpoint := os.Getenv("ENTE_TEST_TURSO_URL")
	if endpoint == "" {
		t.Skip("requires a disposable libSQL database")
	}
	ctx := context.Background()
	cfg := Config{Provider: Turso, TursoURL: endpoint}
	first, err := Open(ctx, cfg)
	require.NoError(t, err)
	defer first.Close()
	second, err := Open(ctx, cfg)
	require.NoError(t, err)
	defer second.Close()
	release, err := acquireMigrationLease(ctx, first)
	require.NoError(t, err)
	defer release()
	_, err = acquireMigrationLease(ctx, second)
	require.ErrorContains(t, err, "another process")
	release()
	releaseSecond, err := acquireMigrationLease(ctx, second)
	require.NoError(t, err)
	releaseSecond()
}
func tableColumns(t *testing.T, db *sql.DB, query string) map[string][]string {
	t.Helper()
	rows, err := db.Query(query)
	require.NoError(t, err)
	defer rows.Close()
	tables := map[string][]string{}
	for rows.Next() {
		var table, col string
		require.NoError(t, rows.Scan(&table, &col))
		tables[table] = append(tables[table], col)
	}
	require.NoError(t, rows.Err())
	for table := range tables {
		sort.Strings(tables[table])
	}
	return tables
}

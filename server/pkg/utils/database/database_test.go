package database

import (
	"bytes"
	"context"
	"database/sql"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestProviderSelection(t *testing.T) {
	for _, provider := range []Provider{"", Postgres, Turso} {
		selected, err := (Config{Provider: provider}).EffectiveProvider()
		require.NoError(t, err)
		if provider == "" {
			provider = Postgres
		}
		require.Equal(t, provider, selected)
	}
	_, err := (Config{Provider: "sqlite"}).EffectiveProvider()
	require.ErrorContains(t, err, "unsupported db.provider")
	require.NoError(t, (Config{}).ValidateMuseumProvider())
	require.NoError(t, (Config{Provider: Turso}).ValidateMuseumProvider())
	require.ErrorContains(t, (Config{Provider: "sqlite"}).ValidateMuseumProvider(), "unsupported db.provider")
}

func TestTursoConfiguration(t *testing.T) {
	cases := []struct {
		name  string
		url   string
		token string
		valid bool
	}{
		{"hosted", "libsql://ente-test.turso.io", "test-secret", true},
		{"https", "https://ente-test.turso.io", "test-secret", true},
		{"local", "http://127.0.0.1:8080", "", true},
		{"ipv6", "http://[::1]:8080", "", true},
		{"localhost", "http://localhost:8080", "", true},
		{"empty", "", "", false},
		{"missing host", "libsql:///database", "test-secret", false},
		{"wrong engine", "turso://ente-test.turso.io", "test-secret", false},
		{"missing token", "libsql://ente-test.turso.io", "", false},
		{"plaintext remote", "http://ente-test.turso.io", "test-secret", false},
		{"userinfo", "https://user:test-secret@ente-test.turso.io", "", false},
		{"query token", "libsql://ente-test.turso.io?authToken=test-secret", "", false},
		{"invalid escape", "libsql://ente-test.turso.io/%test-secret", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := Config{Provider: Turso, TursoURL: tc.url, TursoAuthToken: tc.token}
			err := cfg.validateTursoURL()
			if tc.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.NotContains(t, err.Error(), "test-secret")
				db, openErr := Open(context.Background(), cfg)
				require.Error(t, openErr)
				require.Nil(t, db)
			}
		})
	}
}

func TestConfigEnvironmentOverrides(t *testing.T) {
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.AutomaticEnv()
	viper.SetEnvPrefix("ENTE")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	viper.Set("db.host", "localhost")
	viper.Set("db.turso.url", "http://localhost:8080")
	t.Setenv("ENTE_DB_PROVIDER", "turso")
	t.Setenv("ENTE_DB_TURSO_AUTH_TOKEN", "test-token")
	cfg := ConfigFromViper()
	require.Equal(t, Turso, cfg.Provider)
	require.Equal(t, "test-token", cfg.TursoAuthToken)
	require.Equal(t, "http://localhost:8080", cfg.TursoURL)
}

func TestMissingMigrations(t *testing.T) {
	// A missing baseline must fail before attempting any connection or creating tables.
	err := Migrate(context.Background(), Config{Provider: Turso}, t.TempDir())
	require.ErrorContains(t, err, "no sqlite migrations")
}

func TestTursoHTTPMigrationsAndTransactions(t *testing.T) {
	testURL := os.Getenv("ENTE_TEST_TURSO_URL")
	if testURL == "" {
		t.Skip("requires a disposable local libSQL server in ENTE_TEST_TURSO_URL")
	}
	u, err := url.Parse(testURL)
	require.NoError(t, err)
	ip := net.ParseIP(u.Hostname())
	require.True(t, u.Scheme == "http" && (u.Hostname() == "localhost" || ip != nil && ip.IsLoopback()), "integration tests require a disposable loopback HTTP server")
	cfg := Config{Provider: Turso, TursoURL: testURL}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db, err := Open(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`DROP TABLE IF EXISTS db_probe; DROP TABLE IF EXISTS migration_rollback_probe; DROP TABLE IF EXISTS db_probe_migrations;`)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = db.Exec(`DROP TABLE IF EXISTS db_probe; DROP TABLE IF EXISTS migration_rollback_probe; DROP TABLE IF EXISTS db_probe_migrations;`)
	})

	const integer int64 = 1<<62 + 123
	blob := []byte{0, 255, 128, 1}
	var actualInteger int64
	var actualBlob []byte
	var actualNull sql.NullString
	var actualBool bool
	var repeatedInteger int64
	err = db.QueryRowContext(ctx, `SELECT ?1, ?2, ?3, ?4, ?1`, integer, blob, nil, true).
		Scan(&actualInteger, &actualBlob, &actualNull, &actualBool, &repeatedInteger)
	require.NoError(t, err)
	require.Equal(t, integer, actualInteger)
	require.Equal(t, integer, repeatedInteger)
	require.True(t, bytes.Equal(blob, actualBlob))
	require.False(t, actualNull.Valid)
	require.True(t, actualBool)

	root := t.TempDir()
	directory := filepath.Join(root, "sqlite")
	require.NoError(t, os.Mkdir(directory, 0700))
	require.NoError(t, os.WriteFile(filepath.Join(directory, "1_probe.up.sql"), []byte(`
		CREATE TABLE db_probe (id INTEGER PRIMARY KEY, value BLOB NOT NULL);
		CREATE INDEX db_probe_value ON db_probe(value);
		CREATE TRIGGER db_probe_positive BEFORE INSERT ON db_probe BEGIN
			SELECT CASE WHEN NEW.id < 0 THEN RAISE(ABORT, 'negative;id') END;
		END;
	`), 0600))
	require.NoError(t, migrateWithTable(ctx, cfg, root, "db_probe_migrations"))
	require.NoError(t, migrateWithTable(ctx, cfg, root, "db_probe_migrations"))

	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `INSERT INTO db_probe (id, value) VALUES (?, ?)`, integer, blob)
	require.NoError(t, err)
	require.NoError(t, tx.Rollback())
	var count int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM db_probe`).Scan(&count))
	require.Zero(t, count)

	tx, err = db.BeginTx(ctx, nil)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `INSERT INTO db_probe (id, value) VALUES (?, ?)`, integer, blob)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())
	require.NoError(t, db.QueryRowContext(ctx, `SELECT value FROM db_probe WHERE id = ?`, integer).Scan(&actualBlob))
	require.Equal(t, blob, actualBlob)
	_, err = db.ExecContext(ctx, `INSERT INTO db_probe(id, value) VALUES (-1, X'01')`)
	require.ErrorContains(t, err, "negative;id")

	require.NoError(t, os.WriteFile(filepath.Join(directory, "2_failing.up.sql"), []byte(`
		CREATE TABLE migration_rollback_probe (id INTEGER PRIMARY KEY);
		INSERT INTO db_probe(id, value) VALUES (2, X'01');
		INSERT INTO missing_probe_table(id) VALUES (1);
	`), 0600))
	require.Error(t, migrateWithTable(ctx, cfg, root, "db_probe_migrations"))
	require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM db_probe WHERE id = 2`).Scan(&count))
	require.Zero(t, count, "a failed migration must roll back its earlier statements")
	require.NoError(t, db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE name = 'migration_rollback_probe'`).Scan(&count))
	require.Zero(t, count, "a failed migration must also roll back schema changes")
	var version int
	var dirty bool
	require.NoError(t, db.QueryRowContext(ctx, `SELECT version, dirty FROM db_probe_migrations`).Scan(&version, &dirty))
	require.Equal(t, 2, version)
	require.True(t, dirty)
	require.ErrorContains(t, migrateWithTable(ctx, cfg, root, "db_probe_migrations"), "Dirty database")

	canceled, cancelNow := context.WithCancel(context.Background())
	cancelNow()
	_, err = db.ExecContext(canceled, `INSERT INTO db_probe(id, value) VALUES (3, X'01')`)
	require.ErrorIs(t, err, context.Canceled)
}

func TestPostgresMigrationTrack(t *testing.T) {
	dsn := os.Getenv("ENTE_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("requires a disposable PostgreSQL database in ENTE_TEST_POSTGRES_DSN")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cfg := Config{PostgresDSN: dsn}
	db, err := Open(ctx, cfg)
	require.NoError(t, err)
	defer db.Close()
	var name string
	require.NoError(t, db.QueryRowContext(ctx, `SELECT current_database()`).Scan(&name))
	require.True(t, strings.HasPrefix(name, "ente_test_"), "refusing to migrate a non-test PostgreSQL database")
	root := filepath.Join("..", "..", "..", "migrations")
	require.NoError(t, Migrate(ctx, cfg, root))
	require.NoError(t, Migrate(ctx, cfg, root))
	var version int
	var dirty bool
	require.NoError(t, db.QueryRowContext(ctx, `SELECT version, dirty FROM schema_migrations`).Scan(&version, &dirty))
	require.Equal(t, 154, version)
	require.False(t, dirty)
}

package config

import (
	"context"
	"database/sql"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestPostgresConfigPreservesDatabaseWithPasswordCharacters(t *testing.T) {
	host := os.Getenv("ENTE_CONFIG_TEST_PG_HOST")
	name := os.Getenv("ENTE_CONFIG_TEST_PG_NAME")
	if host == "" || name == "" {
		t.Skip("requires a disposable PostgreSQL instance with trust authentication")
	}
	require.True(t, net.ParseIP(host).IsLoopback())
	require.True(t, strings.HasPrefix(name, "ente_test_"))
	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.Set("db.host", host)
	viper.Set("db.port", os.Getenv("ENTE_CONFIG_TEST_PG_PORT"))
	viper.Set("db.user", "postgres")
	viper.Set("db.name", name)
	viper.Set("db.sslmode", "disable")
	for _, password := range []string{"", "spaces dbname=postgres", "quote'and\\backslash"} {
		t.Run(password, func(t *testing.T) {
			viper.Set("db.password", password)
			db, err := sql.Open("postgres", GetPGInfo())
			require.NoError(t, err)
			defer db.Close()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			var actual string
			require.NoError(t, db.QueryRowContext(ctx, "SELECT current_database()").Scan(&actual))
			require.Equal(t, name, actual)
		})
	}
}

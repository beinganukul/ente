package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/ente/museum/pkg/utils/config"
	_ "github.com/lib/pq"
	"github.com/spf13/viper"
	"github.com/tursodatabase/libsql-client-go/libsql"
)

type Provider string

const (
	Postgres Provider = "postgres"
	Turso    Provider = "turso"
)

type Config struct {
	Provider       Provider
	PostgresDSN    string
	TursoURL       string
	TursoAuthToken string
}

func ConfigFromViper() Config {
	return Config{
		Provider:       Provider(viper.GetString("db.provider")),
		PostgresDSN:    config.GetPGInfo(),
		TursoURL:       viper.GetString("db.turso.url"),
		TursoAuthToken: viper.GetString("db.turso.auth-token"),
	}
}

func (c Config) EffectiveProvider() (Provider, error) {
	switch c.Provider {
	case "", Postgres:
		return Postgres, nil
	case Turso:
		return Turso, nil
	default:
		return "", fmt.Errorf("unsupported db.provider %q", c.Provider)
	}
}

func (c Config) validateTursoURL() error {
	u, err := url.Parse(c.TursoURL)
	if err != nil || u.Hostname() == "" {
		return errors.New("db.turso.url must be a valid libsql:// or https:// URL")
	}
	// Tokens belong in a separate setting so connection errors cannot expose them.
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return errors.New("db.turso.url must not contain credentials, query parameters, or a fragment; use db.turso.auth-token")
	}
	loopback := strings.EqualFold(u.Hostname(), "localhost")
	if ip := net.ParseIP(u.Hostname()); ip != nil {
		loopback = ip.IsLoopback()
	}
	switch u.Scheme {
	case "libsql", "https":
	case "http":
		if !loopback {
			return errors.New("unencrypted Turso HTTP connections are only allowed on loopback addresses")
		}
	default:
		return errors.New("db.turso.url must use libsql:// or https://; http:// is supported for local development")
	}
	if !loopback && c.TursoAuthToken == "" {
		return errors.New("db.turso.auth-token is required for a remote Turso database")
	}
	return nil
}

func Open(ctx context.Context, cfg Config) (*sql.DB, error) {
	provider, err := cfg.EffectiveProvider()
	if err != nil {
		return nil, err
	}
	var db *sql.DB
	switch provider {
	case Postgres:
		db, err = sql.Open("postgres", cfg.PostgresDSN)
	case Turso:
		if err := cfg.validateTursoURL(); err != nil {
			return nil, err
		}
		var options []libsql.Option
		if cfg.TursoAuthToken != "" {
			options = append(options, libsql.WithAuthToken(cfg.TursoAuthToken))
		}
		connector, connectorErr := libsql.NewConnector(cfg.TursoURL, options...)
		if connectorErr != nil {
			return nil, fmt.Errorf("create Turso connector: %w", connectorErr)
		}
		db = sql.OpenDB(tursoConnector{Connector: connector, stats: &tursoStats{}})
	}
	if err != nil {
		return nil, fmt.Errorf("open %s database: %w", provider, err)
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping %s database: %w", provider, err)
	}
	return db, nil
}

func (c Config) ValidateMuseumProvider() error {
	_, err := c.EffectiveProvider()
	return err
}

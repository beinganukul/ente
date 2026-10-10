//go:build js && wasm

package database

import (
	"context"
	"errors"
)

func Migrate(context.Context, Config, string) error {
	return errors.New("run database migrations with the native cmd/database before deploying the Worker")
}

//go:build js && wasm

package utils

import (
	"context"
	"fmt"
	"time"

	"github.com/syumai/workers-go/exp/cloudflare/durableobjects"
	"github.com/ulule/limiter/v3"
	"github.com/ulule/limiter/v3/drivers/store/common"
)

type memoryLimiterStore struct{}

func newMemoryLimiterStore() *memoryLimiterStore { return &memoryLimiterStore{} }

func (store *memoryLimiterStore) operation(key string, rate limiter.Rate, increment int, reset bool) (limiter.Context, error) {
	now := time.Now()
	sql := durableobjects.State().Storage().SQL()
	if _, err := sql.Exec("CREATE TABLE IF NOT EXISTS museum_rate_limits (key TEXT PRIMARY KEY, count INTEGER NOT NULL, expires INTEGER NOT NULL)"); err != nil {
		return limiter.Context{}, err
	}
	if _, err := sql.Exec("CREATE INDEX IF NOT EXISTS museum_rate_limits_expiry ON museum_rate_limits(expires)"); err != nil {
		return limiter.Context{}, err
	}
	if _, err := sql.Exec("DELETE FROM museum_rate_limits WHERE expires <= ?", now.UnixMilli()); err != nil {
		return limiter.Context{}, err
	}
	if reset {
		if _, err := sql.Exec("DELETE FROM museum_rate_limits WHERE key = ?", key); err != nil {
			return limiter.Context{}, err
		}
	}
	rows, err := sql.Exec("INSERT INTO museum_rate_limits(key, count, expires) VALUES (?, ?, ?) ON CONFLICT(key) DO UPDATE SET count = count + excluded.count RETURNING count, expires", key, increment, now.Add(rate.Period).UnixMilli())
	if err != nil {
		return limiter.Context{}, err
	}
	row, err := rows.One()
	if err != nil {
		return limiter.Context{}, fmt.Errorf("read rate limit: %w", err)
	}
	return common.GetContextFromState(now, rate, time.UnixMilli(int64(row.Get("expires").Float())), int64(row.Get("count").Float())), nil
}
func (store *memoryLimiterStore) Get(_ context.Context, key string, rate limiter.Rate) (limiter.Context, error) {
	return store.operation(key, rate, 1, false)
}
func (store *memoryLimiterStore) Peek(_ context.Context, key string, rate limiter.Rate) (limiter.Context, error) {
	return store.operation(key, rate, 0, false)
}
func (store *memoryLimiterStore) Reset(_ context.Context, key string, rate limiter.Rate) (limiter.Context, error) {
	return store.operation(key, rate, 0, true)
}

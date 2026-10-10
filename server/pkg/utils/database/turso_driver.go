package database

import (
	"context"
	"database/sql/driver"
	"errors"
	"strings"
	"sync/atomic"
	"time"
)

// This adapter changes only numbered parameter tokens. SQL dialect differences
// remain explicit at their call sites. BEGIN IMMEDIATE serializes read/write
// transactions before they observe state that PostgreSQL protects with row locks.
type tursoStats struct{ waiting atomic.Int64 }
type tursoConnector struct {
	driver.Connector
	stats *tursoStats
}

func (c tursoConnector) Driver() driver.Driver { return tursoDriver{stats: c.stats} }
func (c tursoConnector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := c.Connector.Connect(ctx)
	if err != nil {
		return nil, err
	}
	wrapped := &tursoConn{Conn: conn, stats: c.stats}
	if _, err = wrapped.ExecContext(ctx, "PRAGMA foreign_keys=ON", nil); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return wrapped, nil
}

type tursoDriver struct{ stats *tursoStats }

func (tursoDriver) Open(string) (driver.Conn, error) {
	return nil, errors.New("use database.Open for Turso")
}

type tursoConn struct {
	driver.Conn
	stats   *tursoStats
	invalid bool
}

func (c *tursoConn) ExecContext(ctx context.Context, q string, a []driver.NamedValue) (driver.Result, error) {
	result, err := c.Conn.(driver.ExecerContext).ExecContext(ctx, numberedParameters(q), a)
	c.markInvalid(err)
	if expiredHranaStream(err) {
		// The server rejected the expired stream before dispatching SQL. A
		// pooled operation can acquire a new connection; sql.Tx never retries.
		err = errors.Join(driver.ErrBadConn, err)
	}
	return result, err
}
func (c *tursoConn) QueryContext(ctx context.Context, q string, a []driver.NamedValue) (driver.Rows, error) {
	rows, err := c.Conn.(driver.QueryerContext).QueryContext(ctx, numberedParameters(q), a)
	c.markInvalid(err)
	if expiredHranaStream(err) {
		err = errors.Join(driver.ErrBadConn, err)
	}
	return rows, err
}
func (c *tursoConn) Prepare(q string) (driver.Stmt, error) {
	return c.Conn.Prepare(numberedParameters(q))
}
func (c *tursoConn) PrepareContext(ctx context.Context, q string) (driver.Stmt, error) {
	return c.Conn.(driver.ConnPrepareContext).PrepareContext(ctx, numberedParameters(q))
}
func (c *tursoConn) Ping(ctx context.Context) error {
	if p, ok := c.Conn.(driver.Pinger); ok {
		return p.Ping(ctx)
	}
	return nil
}
func (c *tursoConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}
func (c *tursoConn) BeginTx(ctx context.Context, o driver.TxOptions) (driver.Tx, error) {
	if o.Isolation != 0 {
		return nil, errors.New("turso supports default transaction isolation only")
	}
	if c.stats != nil {
		c.stats.waiting.Add(1)
		defer c.stats.waiting.Add(-1)
	}
	q := "BEGIN IMMEDIATE"
	if o.ReadOnly {
		q = "BEGIN"
	}
	waitCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	for {
		_, err := c.ExecContext(waitCtx, q, nil)
		if err == nil {
			break
		}
		// A rejected expired Hrana stream means BEGIN was not accepted. Tell
		// database/sql to acquire a new connection; never replay transaction work.
		if c.invalid {
			return nil, errors.Join(driver.ErrBadConn, err)
		}
		if !strings.Contains(err.Error(), "SQLITE_BUSY") && !strings.Contains(err.Error(), "database is locked") {
			// A failed response may leave BEGIN active on the remote stream.
			// Discard that connection without retrying an ambiguous request.
			c.invalid = true
			return nil, err
		}
		select {
		case <-waitCtx.Done():
			c.invalid = true
			return nil, waitCtx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}

	return tursoTx{c}, nil
}

type tursoTx struct{ c *tursoConn }

func (t tursoTx) Commit() error {
	_, err := t.c.ExecContext(context.Background(), "COMMIT", nil)
	if err != nil {
		t.c.invalid = true
	}
	return err
}
func (t tursoTx) Rollback() error {
	_, err := t.c.ExecContext(context.Background(), "ROLLBACK", nil)
	if err != nil {
		t.c.invalid = true
	}
	return err
}

func numberedParameters(q string) string {
	var b strings.Builder
	for i := 0; i < len(q); {
		if q[i] == '\'' || q[i] == '"' || q[i] == '`' {
			quote := q[i]
			start := i
			i++
			for i < len(q) {
				if q[i] == quote {
					i++
					if i < len(q) && q[i] == quote {
						i++
						continue
					}
					break
				}
				i++
			}
			b.WriteString(q[start:i])
			continue
		}
		if i+1 < len(q) && q[i:i+2] == "--" {
			end := strings.IndexByte(q[i:], '\n')
			if end < 0 {
				b.WriteString(q[i:])
				break
			}
			b.WriteString(q[i : i+end])
			i += end
			continue
		}
		if i+1 < len(q) && q[i:i+2] == "/*" {
			end := strings.Index(q[i+2:], "*/")
			if end < 0 {
				b.WriteString(q[i:])
				break
			}
			end += i + 4
			b.WriteString(q[i:end])
			i = end
			continue
		}
		if q[i] == '$' && i+1 < len(q) && q[i+1] >= '0' && q[i+1] <= '9' {
			b.WriteByte('?')
			i++
			continue
		}
		b.WriteByte(q[i])
		i++
	}
	return b.String()
}

// WaitingTransactions reports requests currently acquiring Turso's writer lock.
func WaitingTransactions(db interface{ Driver() driver.Driver }) int64 {
	if d, ok := db.Driver().(tursoDriver); ok && d.stats != nil {
		return d.stats.waiting.Load()
	}
	return 0
}
func (c *tursoConn) markInvalid(err error) {
	if err != nil && (strings.Contains(err.Error(), "invalid baton") || strings.Contains(err.Error(), "stream is closed") || expiredHranaStream(err) || errors.Is(err, driver.ErrBadConn)) {
		c.invalid = true
	}
}

// shortcut: retire each returned Hrana connection until the SDK exposes a reliable stream-state validator.
func (c *tursoConn) IsValid() bool { return false }

func expiredHranaStream(err error) bool {
	if err == nil {
		return false
	}
	message := strings.TrimSpace(err.Error())
	return message == "failed to execute SQL:\nerror code 400:" ||
		strings.HasPrefix(message, "failed to execute SQL:\nerror code 404: stream not found: ")
}

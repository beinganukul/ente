package workerjobs

import (
	"runtime"
	"testing"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/stretchr/testify/require"
)

func TestNextDrainsOverdueJobsBeforeRepeating(t *testing.T) {
	original := Keys
	t.Cleanup(func() { Keys = original })
	Keys = map[cron.EntryID]string{1: "first", 2: "second", 3: "later"}
	now := time.Now()
	next := map[string]time.Time{"first": now.Add(-time.Minute), "second": now.Add(-time.Minute), "later": now.Add(time.Hour)}
	entries := []cron.Entry{{ID: 3}, {ID: 2}, {ID: 1}}
	entry, deadline := Next(entries, next)
	require.Equal(t, cron.EntryID(1), entry.ID)
	require.False(t, deadline.After(now))
	// Advancement after success or panic gives the other overdue job a turn.
	next[Keys[entry.ID]] = now.Add(time.Minute)
	entry, deadline = Next(entries, next)
	require.Equal(t, cron.EntryID(2), entry.ID)
	require.False(t, deadline.After(now))
	next[Keys[entry.ID]] = now.Add(time.Minute)
	_, deadline = Next(entries, next)
	require.True(t, deadline.After(now))
	entry, deadline = Next(entries, nil)
	require.Zero(t, entry.ID)
	require.True(t, deadline.IsZero())
}

func TestBatchSize(t *testing.T) {
	want := 1000
	if runtime.GOOS == "js" {
		want = 5
	}
	require.Equal(t, want, BatchSize(1000, 5))
	require.Equal(t, 3, BatchSize(3, 5))
}

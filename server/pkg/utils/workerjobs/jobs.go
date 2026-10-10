package workerjobs

import (
	"reflect"
	"runtime"
	"time"

	"github.com/robfig/cron/v3"
)

var Scheduler = cron.New()
var Keys = make(map[cron.EntryID]string)
var Immediate = make(map[cron.EntryID]bool)

// BatchSize leaves room for SQL connection setup and storage work within 50 subrequests.
func BatchSize(native, workerLimit int) int {
	if runtime.GOOS == "js" && native > workerLimit {
		return workerLimit
	}
	return native
}

func Add(spec string, step func(), immediate bool) (cron.EntryID, error) {
	id, err := Scheduler.AddFunc(spec, step)
	if err == nil {
		Keys[id] = spec + ":" + runtime.FuncForPC(reflect.ValueOf(step).Pointer()).Name()
		Immediate[id] = immediate
	}
	return id, err
}

// Next selects the oldest deadline, so one job per alarm cannot starve later entries.
func Next(entries []cron.Entry, next map[string]time.Time) (cron.Entry, time.Time) {
	var selected cron.Entry
	var deadline time.Time
	for _, entry := range entries {
		scheduled, exists := next[Keys[entry.ID]]
		if exists && (selected.ID == 0 || scheduled.Before(deadline) || (scheduled.Equal(deadline) && entry.ID < selected.ID)) {
			selected, deadline = entry, scheduled
		}
	}
	return selected, deadline
}

// Register replaces a perpetual worker with one bounded step per alarm.
func Register(spec string, step func()) bool {
	if runtime.GOOS != "js" {
		return false
	}
	if _, err := Add(spec, step, false); err != nil {
		panic(err)
	}
	return true
}

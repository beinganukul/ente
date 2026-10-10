package data_cleanup

import (
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"

	cleanupentity "github.com/ente/museum/ente/data_cleanup"
	"github.com/ente/museum/internal/testutil"
	"github.com/ente/museum/pkg/repo"
	cleanuprepo "github.com/ente/museum/pkg/repo/datacleanup"
	"github.com/ente/museum/pkg/utils/database"
	spacerepo "github.com/ente/museum/space/repo"
)

type cleanupBudgetTransport struct {
	base  http.RoundTripper
	count atomic.Int32
}

func (c *cleanupBudgetTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	if c.count.Add(1) > 50 {
		return nil, fmt.Errorf("Workers Free subrequest limit exceeded")
	}
	return c.base.RoundTrip(request)
}

func TestAccountCleanupCompletesWithinWorkerSubrequestLimit(t *testing.T) {
	testutil.WithServerRoot(t)
	db := testutil.RequireTestDB(t)
	if !database.IsTurso(db) {
		t.Skip("HTTP subrequest accounting requires disposable libSQL")
	}
	testutil.ResetTables(t, db)
	t.Cleanup(func() {
		if _, err := db.Exec(`DELETE FROM data_cleanup`); err != nil {
			t.Error(err)
		}
		testutil.ResetTables(t, db)
	})
	userID := testutil.InsertUser(t, db, testutil.UserFixture{UserID: 1, Email: "cleanup@example.com", CreationTime: 1})
	testutil.InsertUsage(t, db, userID, 0)
	if _, err := db.Exec(`INSERT INTO data_cleanup(user_id, stage, stage_schedule_time) VALUES($1, 'storage', 0)`, userID); err != nil {
		t.Fatal(err)
	}
	controller := &DeleteUserCleanupController{
		Repo:          &cleanuprepo.Repository{DB: db},
		TaskLockRepo:  &repo.TaskLockRepository{DB: db},
		UsageRepo:     &repo.UsageRepository{DB: db},
		SpaceDataRepo: spacerepo.NewModule(db, nil),
		HostName:      "test",
	}
	original := http.DefaultTransport
	budget := &cleanupBudgetTransport{base: original}
	http.DefaultTransport = budget
	t.Cleanup(func() { http.DefaultTransport = original })
	controller.DeleteDataCron()
	http.DefaultTransport = original
	var stage string
	if err := db.QueryRow(`SELECT stage FROM data_cleanup WHERE user_id=$1`, userID).Scan(&stage); err != nil {
		t.Fatal(err)
	}
	if stage != string(cleanupentity.Completed) || budget.count.Load() > 50 {
		t.Fatalf("cleanup stage=%s, subrequests=%d; want completed within 50", stage, budget.count.Load())
	}
	t.Logf("complete account cleanup: %d HTTP subrequests", budget.count.Load())
}

func TestStartCleanupCancelsRecoveredAccount(t *testing.T) {
	testutil.WithServerRoot(t)
	db := testutil.RequireTestDB(t)
	testutil.ResetTables(t, db)
	t.Cleanup(func() {
		if _, err := db.Exec(`DELETE FROM data_cleanup`); err != nil {
			t.Errorf("failed to clear data_cleanup: %v", err)
		}
		testutil.ResetTables(t, db)
	})

	tests := []struct {
		name             string
		cleanupRowExists bool
	}{
		{name: "stale fetched row"},
		{name: "legacy cleanup row", cleanupRowExists: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := db.Exec(`DELETE FROM data_cleanup`); err != nil {
				t.Fatalf("failed to clear data_cleanup: %v", err)
			}
			testutil.ResetTables(t, db)

			userID := testutil.InsertUser(t, db, testutil.UserFixture{
				UserID:       82,
				Email:        "recovered@example.com",
				CreationTime: 1,
			})
			if tt.cleanupRowExists {
				if _, err := db.Exec(`INSERT INTO data_cleanup(user_id) VALUES($1)`, userID); err != nil {
					t.Fatalf("failed to insert cleanup row: %v", err)
				}
			}
			controller := &DeleteUserCleanupController{
				Repo: &cleanuprepo.Repository{DB: db},
				UserRepo: &repo.UserRepository{
					DB:                  db,
					SecretEncryptionKey: testutil.SecretEncryptionKey(),
				},
			}

			if err := controller.startCleanup(t.Context(), &cleanupentity.DataCleanup{
				UserID: userID,
				Stage:  cleanupentity.Scheduled,
			}); err != nil {
				t.Fatalf("startCleanup() error = %v", err)
			}
			var cleanupRows int
			if err := db.QueryRow(`SELECT count(*) FROM data_cleanup WHERE user_id = $1`, userID).Scan(&cleanupRows); err != nil {
				t.Fatalf("failed to count cleanup rows: %v", err)
			}
			if cleanupRows != 0 {
				t.Fatalf("cleanup row count = %d, want 0", cleanupRows)
			}
		})
	}
}

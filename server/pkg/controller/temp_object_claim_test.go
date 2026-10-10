package controller

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/ente/museum/ente"
	"github.com/ente/museum/internal/testutil"
)

func TestClaimedCleanupDoesNotHoldDatabaseLockDuringStorageRequest(t *testing.T) {
	deleting := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	s3Server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodDelete:
			close(deleting)
			<-release
			w.WriteHeader(http.StatusNoContent)
		case http.MethodHead:
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	t.Cleanup(s3Server.Close)
	cleanup, fileRepo, db := setupObjectCleanupRaceTest(t, s3Server.URL)
	t.Cleanup(unblock)
	owner := testutil.InsertUser(t, db, testutil.UserFixture{UserID: 1, Email: "claimed-cleanup@example.com", CreationTime: 1})
	collection := insertObjectCleanupTestCollection(t, db, owner)
	const fileKey, thumbnailKey = "1/claimed-file", "1/claimed-thumb"
	if _, err := db.Exec(`INSERT INTO temp_objects(object_key,expiration_time,bucket_id) VALUES($1,0,'b2-eu-cen'),($2,0,'b2-eu-cen')`, fileKey, thumbnailKey); err != nil {
		t.Fatal(err)
	}
	done := make(chan int, 1)
	go func() { done <- cleanup.removeClaimedUnreportedObjects() }()
	select {
	case <-deleting:
	case <-time.After(3 * time.Second):
		t.Fatal("cleanup did not reach storage")
	}
	// A competing write must succeed while object storage is blocked.
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	if _, err := db.ExecContext(ctx, `UPDATE users SET creation_time = 2 WHERE user_id = $1`, owner); err != nil {
		t.Fatalf("storage request held a database transaction: %v", err)
	}
	_, _, err := fileRepo.Create(objectCleanupTestFile(owner, collection, fileKey, thumbnailKey), 100, 10, 110, owner, ente.Photos)
	var apiErr *ente.ApiError
	if !errors.As(err, &apiErr) || apiErr.Message != "staged upload not found" {
		t.Fatalf("upload registered an object claimed for cleanup: %v", err)
	}
	// Release storage before waiting, retaining safe cleanup if the test fails.
	unblock()
	select {
	case count := <-done:
		if count != 1 {
			t.Fatalf("removed %d objects, want one bounded claim", count)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("cleanup did not finish")
	}
}

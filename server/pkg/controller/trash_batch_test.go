package controller

import (
	"testing"

	"github.com/ente/museum/ente"
	"github.com/ente/museum/internal/testutil"
	"github.com/ente/museum/pkg/repo"
)

func TestEmptyTrashRetainsQueueUntilBoundedBatchesComplete(t *testing.T) {
	testutil.WithServerRoot(t)
	db := testutil.RequireTestDB(t)
	testutil.ResetTables(t, db)
	t.Cleanup(func() { testutil.ResetTables(t, db) })
	ownerID := testutil.InsertUser(t, db, testutil.UserFixture{UserID: 1, Email: "trash-batch@example.com", CreationTime: 1})
	testutil.InsertUsage(t, db, ownerID, 0)
	collectionID := insertObjectCleanupTestCollection(t, db, ownerID)
	for range 10 {
		var fileID int64
		if err := db.QueryRow(`INSERT INTO files(owner_id, file_decryption_header, thumbnail_decryption_header, metadata_decryption_header, encrypted_metadata, updation_time)
			VALUES($1, 'file', 'thumbnail', 'metadata', 'encrypted', 1) RETURNING file_id`, ownerID).Scan(&fileID); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO collection_files(collection_id, file_id, encrypted_key, key_decryption_nonce, is_deleted, updation_time, c_owner_id, f_owner_id)
			VALUES($1, $2, 'key', 'nonce', TRUE, 1, $3, $3)`, collectionID, fileID, ownerID); err != nil {
			t.Fatal(err)
		}
		if _, err := db.Exec(`INSERT INTO trash(file_id, collection_id, user_id, created_at, updated_at, delete_by)
			VALUES($1, $2, $3, 1, 1, 1)`, fileID, collectionID, ownerID); err != nil {
			t.Fatal(err)
		}
	}
	queue := &repo.QueueRepository{DB: db}
	objects := &repo.ObjectRepository{DB: db, QueueRepo: queue}
	trash := &repo.TrashRepository{DB: db, FileRepo: &repo.FileRepository{DB: db, ObjectRepo: objects}, QueueRepo: queue}
	if err := trash.EmptyTrash(t.Context(), ownerID, 1, ente.Photos); err != nil {
		t.Fatal(err)
	}
	if err := trash.EmptyTrash(t.Context(), ownerID, 1, ente.Locker); err != nil {
		t.Fatal(err)
	}
	items, err := queue.GetItemsReadyForDeletion(repo.TrashEmptyQueue, 1)
	if err != nil || len(items) != 1 {
		t.Fatalf("empty trash queue = %v, error = %v", items, err)
	}
	controller := &TrashController{TrashRepo: trash, QueueRepo: queue, TaskLockRepo: &repo.TaskLockRepository{DB: db}, HostName: "test"}
	for step, expected := range []struct{ remaining, queued int }{{5, 1}, {0, 1}, {0, 0}} {
		controller.processEmptyTrashRequests(5)
		var remaining, queued int
		if err := db.QueryRow(`SELECT COUNT(*) FROM trash WHERE is_deleted=FALSE`).Scan(&remaining); err != nil {
			t.Fatal(err)
		}
		if err := db.QueryRow(`SELECT COUNT(*) FROM queue WHERE queue_name=$1 AND is_deleted=FALSE`, repo.TrashEmptyQueue).Scan(&queued); err != nil {
			t.Fatal(err)
		}
		if remaining != expected.remaining || queued != expected.queued {
			t.Fatalf("step %d: remaining=%d queued=%d, want remaining=%d queued=%d", step, remaining, queued, expected.remaining, expected.queued)
		}
		lockerItems, err := queue.GetItemsReadyForDeletion(repo.TrashEmptyLockerQueue, 1)
		if err != nil || len(lockerItems) != 1 {
			t.Fatalf("step %d processed a second app queue in the same invocation: %v, %v", step, lockerItems, err)
		}
	}
	controller.processEmptyTrashRequests(5)
	lockerItems, err := queue.GetItemsReadyForDeletion(repo.TrashEmptyLockerQueue, 1)
	if err != nil || len(lockerItems) != 0 {
		t.Fatalf("locker queue did not drain after photos: %v, %v", lockerItems, err)
	}
}

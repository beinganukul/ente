package filedata

import (
	"fmt"
	"testing"

	"github.com/ente/museum/ente"
	enteFileData "github.com/ente/museum/ente/filedata"
	"github.com/ente/museum/internal/testutil"
	"github.com/ente/museum/pkg/repo"
	"github.com/ente/museum/pkg/utils/database"
	"github.com/stretchr/testify/require"
)

func TestOverwriteRetainsOnlyStaleDeletionBuckets(t *testing.T) {
	testutil.WithServerRoot(t)
	db := testutil.RequireTestDB(t)
	for _, preview := range []bool{false, true} {
		for _, latest := range []string{"b2-eu-cen", "b5"} {
			t.Run(fmt.Sprintf("preview=%t/latest=%s", preview, latest), func(t *testing.T) {
				_, err := db.Exec(`DELETE FROM file_data`)
				require.NoError(t, err)
				testutil.ResetTables(t, db)
				t.Cleanup(func() {
					if _, err := db.Exec(`DELETE FROM file_data`); err != nil {
						t.Error(err)
					}
					testutil.ResetTables(t, db)
				})
				r := &Repository{DB: db, ObjectCleanupRepo: &repo.ObjectCleanupRepository{DB: db}}
				typeOfData := ente.MlData
				if preview {
					typeOfData = ente.PreviewVideo
				}
				_, err = db.Exec(`INSERT INTO file_data(file_id,user_id,data_type,size,latest_bucket,replicated_buckets,delete_from_buckets)
					VALUES(1,10,$1,11,'b2-eu-cen',$2,$3)`, string(typeOfData), database.Array(db, []string{"b5"}), database.Array(db, []string{"b6"}))
				require.NoError(t, err)
				objectID, objectSize := "new-preview", int64(12)
				row := enteFileData.Row{FileID: 1, UserID: 10, Type: typeOfData, Size: 42, LatestBucket: latest, ObjectID: &objectID, ObjectSize: &objectSize}
				if preview {
					require.NoError(t, r.ObjectCleanupRepo.AddTempObject(ente.TempObject{ObjectKey: objectID, BucketId: latest}, 42))
					require.NoError(t, r.InsertOrUpdatePreviewData(t.Context(), row, objectID))
					var staged int
					require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM temp_objects WHERE object_key=$1`, objectID).Scan(&staged))
					require.Zero(t, staged)
				} else {
					require.NoError(t, r.InsertOrUpdate(t.Context(), row))
				}
				actual, err := r.GetFileData(t.Context(), 1)
				require.NoError(t, err)
				require.Len(t, actual, 1)
				require.Equal(t, int64(42), actual[0].Size)
				require.Equal(t, latest, actual[0].LatestBucket)
				require.Empty(t, actual[0].ReplicatedBuckets)
				require.True(t, actual[0].PendingSync)
				stale := "b5"
				if latest == "b5" {
					stale = "b2-eu-cen"
				}
				require.ElementsMatch(t, []string{stale, "b6"}, actual[0].DeleteFromBuckets)
				if preview {
					require.Equal(t, &objectID, actual[0].ObjectID)
				}
			})
		}
	}
}

func TestInsertOrUpdatePreviewDataRequiresObjectID(t *testing.T) {
	if err := (&Repository{}).InsertOrUpdatePreviewData(t.Context(), enteFileData.Row{}, ""); err == nil {
		t.Fatal("InsertOrUpdatePreviewData() error = nil")
	}
}

func TestDeleteFileData(t *testing.T) {
	testutil.WithServerRoot(t)

	db := testutil.RequireTestDB(t)
	if _, err := db.Exec(`DELETE FROM file_data`); err != nil {
		t.Fatalf("failed to reset file_data: %v", err)
	}
	t.Cleanup(func() {
		if _, err := db.Exec(`DELETE FROM file_data`); err != nil {
			t.Errorf("failed to reset file_data: %v", err)
		}
	})

	repo := &Repository{DB: db}

	t.Run("deletes eligible row", func(t *testing.T) {
		row := enteFileData.Row{
			FileID:       1,
			UserID:       10,
			Type:         ente.MlData,
			LatestBucket: "b2-eu-cen",
		}
		insertFileDataForDeleteTest(t, repo, row, true)

		if err := repo.DeleteFileData(t.Context(), row); err != nil {
			t.Fatalf("DeleteFileData() error = %v", err)
		}
		assertFileDataExists(t, repo, row, false)
	})

	t.Run("succeeds when row is already absent", func(t *testing.T) {
		row := enteFileData.Row{
			FileID:       2,
			UserID:       10,
			Type:         ente.MlData,
			LatestBucket: "b2-eu-cen",
		}

		if err := repo.DeleteFileData(t.Context(), row); err != nil {
			t.Fatalf("DeleteFileData() error = %v", err)
		}
	})

	t.Run("rejects row that is not eligible for deletion", func(t *testing.T) {
		row := enteFileData.Row{
			FileID:       3,
			UserID:       10,
			Type:         ente.MlData,
			LatestBucket: "b2-eu-cen",
		}
		insertFileDataForDeleteTest(t, repo, row, false)

		if err := repo.DeleteFileData(t.Context(), row); err == nil {
			t.Fatal("DeleteFileData() error = nil, want ineligible-row error")
		}
		assertFileDataExists(t, repo, row, true)
	})
}

func insertFileDataForDeleteTest(t *testing.T, repo *Repository, row enteFileData.Row, isDeleted bool) {
	t.Helper()
	_, err := repo.DB.ExecContext(t.Context(), `
		INSERT INTO file_data (file_id, user_id, data_type, size, latest_bucket, is_deleted)
		VALUES ($1, $2, $3, 1, $4, $5)`,
		row.FileID, row.UserID, string(row.Type), row.LatestBucket, isDeleted)
	if err != nil {
		t.Fatalf("failed to insert file_data: %v", err)
	}
}

func assertFileDataExists(t *testing.T, repo *Repository, row enteFileData.Row, want bool) {
	t.Helper()
	var got bool
	err := repo.DB.QueryRowContext(t.Context(), `SELECT EXISTS (
		SELECT 1 FROM file_data WHERE file_id = $1 AND data_type = $2
	)`, row.FileID, string(row.Type)).Scan(&got)
	if err != nil {
		t.Fatalf("failed to check file_data: %v", err)
	}
	if got != want {
		t.Fatalf("file_data existence = %v, want %v", got, want)
	}
}

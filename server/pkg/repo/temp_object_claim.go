package repo

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/ente/museum/ente"
	"github.com/ente/museum/pkg/utils/database"
	"github.com/google/uuid"
)

// ClaimExpiredObjects commits a bounded cleanup claim before storage I/O.
// A claim remains attached after its lease expires, so upload finalization can
// never register an object that a crashed cleanup worker might have deleted.
func (repo *ObjectCleanupRepository) ClaimExpiredObjects() ([]ente.TempObject, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	tx, err := repo.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, "", err
	}
	defer tx.Rollback()
	now := time.Now().UnixMicro()
	query := `SELECT object_key,is_multipart,upload_id,bucket_id FROM temp_objects
	 WHERE expiration_time <= $1 AND cleanup_claim_until <= $1
	 AND NOT EXISTS(SELECT 1 FROM object_keys WHERE object_keys.object_key = temp_objects.object_key)
	 ORDER BY expiration_time,object_key LIMIT 1`
	query = database.SQL(repo.DB, query+` FOR UPDATE SKIP LOCKED`, query)
	var object ente.TempObject
	var uploadID, bucketID sql.NullString
	err = tx.QueryRowContext(ctx, query, now).Scan(&object.ObjectKey, &object.IsMultipart, &uploadID, &bucketID)
	if err == sql.ErrNoRows {
		return nil, "", nil
	}
	if err != nil {
		return nil, "", err
	}
	object.UploadID, object.BucketId = uploadID.String, bucketID.String
	claim := uuid.NewString()
	_, err = tx.ExecContext(ctx, `UPDATE temp_objects SET cleanup_claim=$1,cleanup_claim_until=$2 WHERE object_key=$3`,
		claim, now+(30*time.Minute).Microseconds(), object.ObjectKey)
	if err != nil {
		return nil, "", err
	}
	if err := tx.Commit(); err != nil {
		return nil, "", err
	}
	return []ente.TempObject{object}, claim, nil
}

func (repo *ObjectCleanupRepository) FinishClaimedObject(object ente.TempObject, claim string) error {
	result, err := repo.DB.Exec(`DELETE FROM temp_objects WHERE object_key=$1 AND cleanup_claim=$2`, object.ObjectKey, claim)
	return checkCleanupClaimResult(result, err)
}

func (repo *ObjectCleanupRepository) DelayClaimedObject(object ente.TempObject, claim string, expiry int64) error {
	result, err := repo.DB.Exec(`UPDATE temp_objects SET expiration_time=$1,cleanup_claim_until=0
	 WHERE object_key=$2 AND cleanup_claim=$3`, expiry, object.ObjectKey, claim)
	return checkCleanupClaimResult(result, err)
}

func checkCleanupClaimResult(result sql.Result, err error) error {
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count != 1 {
		return fmt.Errorf("cleanup claim no longer owns the temporary object")
	}
	return nil
}

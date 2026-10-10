package repo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/ente/museum/ente"
	"github.com/ente/museum/pkg/utils/database"
	"github.com/ente/stacktrace"
)

type ObjectCopiesRepository struct {
	DB *sql.DB
}

func (repo *ObjectCopiesRepository) GetAndLockUnreplicatedObject(ctx context.Context) (*ente.ObjectCopies, error) {
	tx, err := repo.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, stacktrace.Propagate(err, "")
	}
	defer tx.Rollback()

	row := tx.QueryRowContext(ctx, database.SQL(repo.DB,
		`
	SELECT object_key, want_b2, b2, want_wasabi, wasabi, want_scw, scw
	FROM object_copies
	WHERE (
		(
			(wasabi IS NULL AND want_wasabi = true) OR
			(scw IS NULL AND want_scw = true)
		) AND last_attempt < (now_utc_micro_seconds() - (24::BIGINT * 60 * 60 * 1000 * 1000))
	)
	LIMIT 1
	FOR UPDATE SKIP LOCKED
	`, `
	SELECT object_key, want_b2, b2, want_wasabi, wasabi, want_scw, scw
	FROM object_copies
	WHERE (
		(
			(wasabi IS NULL AND want_wasabi = true) OR
			(scw IS NULL AND want_scw = true)
		) AND last_attempt < (CAST(unixepoch('subsec') * 1000000 AS INTEGER) - (24 * 60 * 60 * 1000 * 1000))
	)
	LIMIT 1
`))

	var r ente.ObjectCopies
	err = row.Scan(&r.ObjectKey, &r.WantB2, &r.B2, &r.WantWasabi, &r.Wasabi,
		&r.WantSCW, &r.SCW)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		return nil, stacktrace.Propagate(err, "")
	}

	err = repo.RegisterReplicationAttempt(tx, ctx, r.ObjectKey)
	if err != nil {
		return nil, stacktrace.Propagate(err, "failed to register replication attempt")
	}

	err = tx.Commit()
	if err != nil {
		return nil, stacktrace.Propagate(err, "")
	}
	return &r, nil
}

func (repo *ObjectCopiesRepository) CreateNewB2Object(ctx context.Context, tx *sql.Tx, objectKey string, wantWasabi bool, wantScaleway bool) error {
	_, err := tx.ExecContext(ctx, database.SQL(repo.DB,
		`
	INSERT INTO object_copies (object_key, want_b2, b2, want_wasabi, want_scw)
	VALUES ($1, true, now_utc_micro_seconds(), $2, $3)
	`, `
	INSERT INTO object_copies (object_key, want_b2, b2, want_wasabi, want_scw)
	VALUES ($1, true, CAST(unixepoch('subsec') * 1000000 AS INTEGER), $2, $3)
`), objectKey, wantWasabi, wantScaleway)
	return stacktrace.Propagate(err, "")
}

func (repo *ObjectCopiesRepository) CreateNewWasabiObject(ctx context.Context, tx *sql.Tx, objectKey string, wantB2 bool, wantScaleway bool) error {
	_, err := tx.ExecContext(ctx, database.SQL(repo.DB,
		`
	INSERT INTO object_copies (object_key, want_wasabi, wasabi, want_b2, want_scw)
	VALUES ($1, true, now_utc_micro_seconds(), $2, $3)
	`, `
	INSERT INTO object_copies (object_key, want_wasabi, wasabi, want_b2, want_scw)
	VALUES ($1, true, CAST(unixepoch('subsec') * 1000000 AS INTEGER), $2, $3)
`), objectKey, wantB2, wantScaleway)
	return stacktrace.Propagate(err, "")
}

func (repo *ObjectCopiesRepository) RegisterReplicationAttempt(tx *sql.Tx, ctx context.Context, objectKey string) error {
	_, err := tx.ExecContext(ctx, database.SQL(repo.DB,
		`
	UPDATE object_copies
	SET last_attempt = now_utc_micro_seconds()
	WHERE object_key = $1
	`, `
	UPDATE object_copies
	SET last_attempt = CAST(unixepoch('subsec') * 1000000 AS INTEGER)
	WHERE object_key = $1
`), objectKey)
	return stacktrace.Propagate(err, "")
}

func (repo *ObjectCopiesRepository) DelayNextAttemptByDays(ctx context.Context, objectKey string, days int) error {
	_, err := repo.DB.ExecContext(ctx, database.SQL(repo.DB,
		`
	UPDATE object_copies
	SET last_attempt = last_attempt + ($2 * 24::BIGINT * 60 * 60 * 1000 * 1000)
	WHERE object_key = $1
	`, `
	UPDATE object_copies
	SET last_attempt = last_attempt + ($2 * 24 * 60 * 60 * 1000 * 1000)
	WHERE object_key = $1
`), objectKey, days)
	return stacktrace.Propagate(err, "")
}

func (repo *ObjectCopiesRepository) ResetNeedsB2Replication(objectKey string) error {
	_, err := repo.DB.Exec(`UPDATE object_copies SET b2 = null WHERE object_key = $1`,
		objectKey)
	return stacktrace.Propagate(err, "")
}

func (repo *ObjectCopiesRepository) ResetNeedsWasabiReplication(objectKey string) error {
	_, err := repo.DB.Exec(`UPDATE object_copies SET wasabi = null WHERE object_key = $1`,
		objectKey)
	return stacktrace.Propagate(err, "")
}

func (repo *ObjectCopiesRepository) ResetNeedsScalewayReplication(objectKey string) error {
	_, err := repo.DB.Exec(`UPDATE object_copies SET scw = null WHERE object_key = $1`,
		objectKey)
	return stacktrace.Propagate(err, "")
}

func (repo *ObjectCopiesRepository) UnmarkFromReplication(objectKey string) error {
	_, err := repo.DB.Exec(`
	UPDATE object_copies
	SET want_b2 = false, want_wasabi = false, want_scw = false
	WHERE object_key = $1
	`, objectKey)
	return stacktrace.Propagate(err, "")
}

func (repo *ObjectCopiesRepository) MarkObjectReplicatedWasabi(objectKey string) error {
	return repo.markObjectReplicated(database.SQL(repo.DB,
		`
	UPDATE object_copies SET wasabi = now_utc_micro_seconds()
	WHERE object_key = $1
	`, `
	UPDATE object_copies SET wasabi = CAST(unixepoch('subsec') * 1000000 AS INTEGER)
	WHERE object_key = $1
`), objectKey)
}

func (repo *ObjectCopiesRepository) MarkObjectReplicatedScaleway(objectKey string) error {
	return repo.markObjectReplicated(database.SQL(repo.DB,
		`
	UPDATE object_copies SET scw = now_utc_micro_seconds()
	WHERE object_key = $1
	`, `
	UPDATE object_copies SET scw = CAST(unixepoch('subsec') * 1000000 AS INTEGER)
	WHERE object_key = $1
`), objectKey)
}

func (repo *ObjectCopiesRepository) markObjectReplicated(query string, objectKey string) error {
	result, err := repo.DB.Exec(query, objectKey)
	if err != nil {
		return stacktrace.Propagate(err, "")
	}
	c, err := result.RowsAffected()
	if err != nil {
		return stacktrace.Propagate(err, "")
	}
	if c != 1 {
		return stacktrace.Propagate(fmt.Errorf("expected 1 row to be updated, but got %d", c), "")
	}
	return nil
}

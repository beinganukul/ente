package repo

import (
	"context"
	"database/sql"

	"github.com/ente/museum/pkg/utils/database"
	"github.com/ente/stacktrace"
)

func lockFiles(ctx context.Context, db *sql.DB, tx *sql.Tx, userID int64, fileIDs []int64) error {
	if len(fileIDs) == 0 {
		return nil
	}

	rows, err := tx.QueryContext(ctx, database.SQL(db,
		`SELECT file_id
		FROM files
		WHERE owner_id = $1 AND file_id = ANY($2)
		ORDER BY file_id
		FOR UPDATE`, `SELECT file_id
		FROM files
		WHERE owner_id = $1 AND file_id IN (SELECT value FROM json_each($2))
		ORDER BY file_id`), userID, database.Array(db,
		fileIDs))
	if err != nil {
		return stacktrace.Propagate(err, "")
	}
	defer rows.Close()

	for rows.Next() {
		var fileID int64
		if err := rows.Scan(&fileID); err != nil {
			return stacktrace.Propagate(err, "")
		}
	}
	return stacktrace.Propagate(rows.Err(), "")
}

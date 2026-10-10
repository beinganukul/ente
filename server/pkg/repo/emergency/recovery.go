package emergency

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/sirupsen/logrus"
	"runtime"

	"github.com/ente/museum/ente"
	"github.com/ente/museum/pkg/utils/database"
	"github.com/ente/museum/pkg/utils/time"
	"github.com/ente/stacktrace"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type RecoverRow struct {
	ID                 uuid.UUID
	UserID             int64
	EmergencyContactID int64
	Status             ente.RecoveryStatus
	WaitTill           int64
	NextReminderAt     int64
	CreatedAt          int64
}

func (r RecoverRow) CanRecover() error {
	if r.Status != ente.RecoveryStatusReady && r.Status != ente.RecoveryStatusWaiting {
		return fmt.Errorf("recovery status is not waiting or ready")
	}
	if r.WaitTill > time.Microseconds() && r.Status == ente.RecoveryStatusWaiting {
		return fmt.Errorf("recovery wait time is not over")
	}
	return nil
}

func (repo *Repository) InsertIntoRecovery(ctx context.Context, contact ente.ContactIdentifier) (bool, *ContactRow, error) {
	tx, err := repo.DB.BeginTx(ctx, nil)
	if err != nil {
		return false, nil, stacktrace.Propagate(err, "failed to start emergency recovery")
	}
	defer tx.Rollback()
	if err = lockOwnerForUpdate(ctx, repo.DB, tx, contact.UserID); err != nil {
		return false, nil, err
	}
	contactRow, err := getContactForUpdate(ctx, repo.DB, tx, contact.UserID, contact.EmergencyContactID)
	if err != nil {
		return false, nil, err
	}
	if contactRow.State != ente.ContactAccepted {
		return false, nil, stacktrace.Propagate(sql.ErrNoRows, "active emergency contact not found")
	}
	if contactRow.NoticePeriodInHrs <= 24 {
		logrus.Warn("notice period is less than 24 hours")
		return false, nil, ente.NewBadRequestWithMessage("notice period should be greater than 24 hours")
	}
	hasActiveSession, err := hasActiveRecovery(ctx, repo.DB, tx, contact.UserID, contact.EmergencyContactID)
	if err != nil {
		return false, nil, err
	}
	if hasActiveSession {
		return false, contactRow, nil
	}
	waitTime := time.MicrosecondsAfterHours(contactRow.NoticePeriodInHrs)
	nextReminder := time.MicrosecondsAfterHours(24 * 7)
	if nextReminder >= waitTime {
		logrus.Warn("initial reminder is greater than wait time")
		nextReminder = time.MicrosecondsAfterHours(24 * 1)
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO emergency_recovery (id,user_id, emergency_contact_id, status, wait_till, next_reminder_at) VALUES ($1, $2, $3, $4, $5, $6)`,
		uuid.New(), contact.UserID, contact.EmergencyContactID, ente.RecoveryStatusWaiting, waitTime, nextReminder)
	if err != nil {
		return false, nil, stacktrace.Propagate(err, "")
	}
	if err = tx.Commit(); err != nil {
		return false, nil, stacktrace.Propagate(err, "failed to commit emergency recovery")
	}
	return true, contactRow, nil
}

func (repo *Repository) GetActiveRecoverySessions(ctx *gin.Context, userID int64) ([]*RecoverRow, error) {
	rows, err := repo.DB.QueryContext(ctx, database.SQL(repo.DB,
		`SELECT id, user_id, emergency_contact_id, status, wait_till, next_reminder_at, created_at
FROM emergency_recovery WHERE (user_id=$1  OR emergency_contact_id=$1) AND status= ANY($2)`, `SELECT id, user_id, emergency_contact_id, status, wait_till, next_reminder_at, created_at
FROM emergency_recovery WHERE (user_id=$1  OR emergency_contact_id=$1) AND status IN (SELECT value FROM json_each($2))`), userID, database.Array(repo.DB,
		[]ente.RecoveryStatus{ente.RecoveryStatusWaiting, ente.RecoveryStatusReady}))
	if err != nil {
		return nil, stacktrace.Propagate(err, "")
	}
	defer rows.Close()
	var sessions []*RecoverRow
	for rows.Next() {
		var row RecoverRow
		if err := rows.Scan(&row.ID, &row.UserID, &row.EmergencyContactID, &row.Status, &row.WaitTill, &row.NextReminderAt, &row.CreatedAt); err != nil {
			return nil, stacktrace.Propagate(err, "")
		}
		sessions = append(sessions, &row)
	}
	return sessions, nil
}

func (r *Repository) GetActiveRecoveryForNotification() (*[]RecoverRow, error) {
	query := database.SQL(r.DB,
		`
SELECT id, user_id, emergency_contact_id, status, wait_till, next_reminder_at, created_at
FROM emergency_recovery WHERE (status = $1) and next_reminder_at < now_utc_micro_seconds()`, `
SELECT id, user_id, emergency_contact_id, status, wait_till, next_reminder_at, created_at
FROM emergency_recovery WHERE (status = $1) and next_reminder_at < CAST(unixepoch('subsec') * 1000000 AS INTEGER)`)
	if runtime.GOOS == "js" {
		query += " ORDER BY next_reminder_at, id LIMIT 1"
	}
	rows, err := r.DB.Query(query, ente.RecoveryStatusWaiting)
	if err != nil {
		return nil, stacktrace.Propagate(err, "")
	}
	defer rows.Close()
	var sessions []RecoverRow
	for rows.Next() {
		var row RecoverRow
		if err := rows.Scan(&row.ID, &row.UserID, &row.EmergencyContactID, &row.Status, &row.WaitTill, &row.NextReminderAt, &row.CreatedAt); err != nil {
			return nil, stacktrace.Propagate(err, "")
		}
		sessions = append(sessions, row)
	}
	return &sessions, nil
}

func (r *Repository) UpdateNextReminder(ctx context.Context, sessionID uuid.UUID, nextReminder int64) error {
	_, err := r.DB.ExecContext(ctx, `UPDATE emergency_recovery SET next_reminder_at=$1 WHERE id=$2`, nextReminder, sessionID)
	if err != nil {
		return stacktrace.Propagate(err, "")
	}
	return nil
}

func (repo *Repository) UpdateRecoveryStatusForID(ctx context.Context, sessionID uuid.UUID, status ente.RecoveryStatus) (bool, error) {
	validPrevStatus := validPreviousStatus(status)
	var result sql.Result
	var err error
	if status == ente.RecoveryStatusReady {
		result, err = repo.DB.ExecContext(ctx, database.SQL(repo.DB,
			`UPDATE emergency_recovery SET status=$1, wait_till=$2 WHERE id=$3 and status = ANY($4)`, `UPDATE emergency_recovery SET status=$1, wait_till=$2 WHERE id=$3 and status IN (SELECT value FROM json_each($4))`), status, time.Microseconds(), sessionID, database.Array(repo.DB,
			validPrevStatus))
	} else {
		result, err = repo.DB.ExecContext(ctx, database.SQL(repo.DB,
			`UPDATE emergency_recovery SET status=$1 WHERE id=$2 and status = ANY($3)`, `UPDATE emergency_recovery SET status=$1 WHERE id=$2 and status IN (SELECT value FROM json_each($3))`), status, sessionID, database.Array(repo.DB,
			validPrevStatus))
	}
	if err != nil {
		return false, stacktrace.Propagate(err, "")
	}
	rows, _ := result.RowsAffected()
	return rows > 0, nil
}

func (repo *Repository) UpdateRecoveryStatusForSession(ctx context.Context, sessionID uuid.UUID, userID, emergencyContactID int64, status ente.RecoveryStatus) (bool, error) {
	validPrevStatus := validPreviousStatus(status)
	var result sql.Result
	var err error
	if status == ente.RecoveryStatusReady {
		result, err = repo.DB.ExecContext(ctx, database.SQL(repo.DB,
			`UPDATE emergency_recovery SET status=$1, wait_till=$2 WHERE id=$3 and user_id=$4 and emergency_contact_id=$5 and status = ANY($6)`, `UPDATE emergency_recovery SET status=$1, wait_till=$2 WHERE id=$3 and user_id=$4 and emergency_contact_id=$5 and status IN (SELECT value FROM json_each($6))`), status, time.Microseconds(), sessionID, userID, emergencyContactID, database.Array(repo.DB,
			validPrevStatus))
	} else {
		result, err = repo.DB.ExecContext(ctx, database.SQL(repo.DB,
			`UPDATE emergency_recovery SET status=$1 WHERE id=$2 and user_id=$3 and emergency_contact_id=$4 and status = ANY($5)`, `UPDATE emergency_recovery SET status=$1 WHERE id=$2 and user_id=$3 and emergency_contact_id=$4 and status IN (SELECT value FROM json_each($5))`), status, sessionID, userID, emergencyContactID, database.Array(repo.DB,
			validPrevStatus))
	}
	if err != nil {
		return false, stacktrace.Propagate(err, "")
	}
	rows, _ := result.RowsAffected()
	return rows > 0, nil
}

func (repo *Repository) CompleteRecovery(ctx context.Context, tx *sql.Tx, sessionID uuid.UUID, userID, emergencyContactID int64) error {
	contact, err := getContactForUpdate(ctx, repo.DB, tx, userID, emergencyContactID)
	if err != nil {
		return err
	}
	if contact.State != ente.ContactAccepted || contact.EncryptedKey == nil {
		return stacktrace.Propagate(ente.ErrNotFound, "active emergency contact not found")
	}
	session, err := getRecoveryForUpdate(ctx, repo.DB, tx, sessionID)
	if err != nil {
		return err
	}
	if session.UserID != userID || session.EmergencyContactID != emergencyContactID {
		return stacktrace.Propagate(ente.ErrPermissionDenied, "recovery session does not match contact")
	}
	if err = session.CanRecover(); err != nil {
		return stacktrace.Propagate(ente.NewBadRequestWithMessage(err.Error()), "")
	}
	_, err = tx.ExecContext(ctx, `UPDATE emergency_recovery SET status=$1 WHERE id=$2`, ente.RecoveryStatusRecovered, sessionID)
	return stacktrace.Propagate(err, "failed to complete emergency recovery")
}

func (repo *Repository) GetRecoverRowByID(ctx context.Context, sessionID uuid.UUID) (*RecoverRow, error) {
	var row RecoverRow
	err := repo.DB.QueryRowContext(ctx, `SELECT id, user_id, emergency_contact_id, status, wait_till, next_reminder_at, created_at
	FROM emergency_recovery WHERE id=$1`, sessionID).Scan(&row.ID, &row.UserID, &row.EmergencyContactID, &row.Status, &row.WaitTill, &row.NextReminderAt, &row.CreatedAt)
	if err != nil {
		return nil, stacktrace.Propagate(err, "")
	}
	return &row, nil
}

func hasActiveRecovery(ctx context.Context, db *sql.DB, tx *sql.Tx, userID, emergencyContactID int64) (bool, error) {
	var exists bool
	err := tx.QueryRowContext(ctx, database.SQL(db,
		`SELECT EXISTS (
		SELECT 1 FROM emergency_recovery WHERE user_id=$1 AND emergency_contact_id=$2 AND status=ANY($3))`, `SELECT EXISTS (
		SELECT 1 FROM emergency_recovery WHERE user_id=$1 AND emergency_contact_id=$2 AND status IN (SELECT value FROM json_each($3)))`), userID, emergencyContactID, database.Array(db,
		[]ente.RecoveryStatus{ente.RecoveryStatusWaiting, ente.RecoveryStatusReady})).Scan(&exists)
	return exists, stacktrace.Propagate(err, "failed to check active recovery sessions")
}

func getRecoveryForUpdate(ctx context.Context, db *sql.DB, tx *sql.Tx, sessionID uuid.UUID) (*RecoverRow, error) {
	var session RecoverRow
	err := tx.QueryRowContext(ctx, database.SQL(db,
		`SELECT id, user_id, emergency_contact_id, status, wait_till, next_reminder_at, created_at
		FROM emergency_recovery WHERE id=$1 FOR UPDATE`, `SELECT id, user_id, emergency_contact_id, status, wait_till, next_reminder_at, created_at
		FROM emergency_recovery WHERE id=$1`), sessionID).
		Scan(&session.ID, &session.UserID, &session.EmergencyContactID, &session.Status, &session.WaitTill, &session.NextReminderAt, &session.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &session, nil
}

func (repo *Repository) UpdateRecoveryStatus(ctx context.Context, userID, emergencyContactID int64, status ente.RecoveryStatus) error {
	validPrevStatus := validPreviousStatus(status)
	_, err := repo.DB.ExecContext(ctx, database.SQL(repo.DB,
		`UPDATE emergency_recovery SET status=$1 WHERE user_id =$2 and emergency_contact_id =$3 and status = ANY($4)`, `UPDATE emergency_recovery SET status=$1 WHERE user_id =$2 and emergency_contact_id =$3 and status IN (SELECT value FROM json_each($4))`), status, userID, emergencyContactID, database.Array(repo.DB,
		validPrevStatus))
	if err != nil {
		return stacktrace.Propagate(err, "")
	}
	return nil
}

func validPreviousStatus(newStatus ente.RecoveryStatus) []ente.RecoveryStatus {
	result := make([]ente.RecoveryStatus, 0)
	switch newStatus {
	case ente.RecoveryStatusWaiting:
		break
	case ente.RecoveryStatusReady:
		result = append(result, ente.RecoveryStatusWaiting, ente.RecoveryStatusReady)
	case ente.RecoveryStatusStopped:
		result = append(result, ente.RecoveryStatusWaiting, ente.RecoveryStatusReady)
	case ente.RecoveryStatusRejected:
		result = append(result, ente.RecoveryStatusWaiting, ente.RecoveryStatusReady)
	case ente.RecoveryStatusRecovered:
		result = append(result, ente.RecoveryStatusWaiting, ente.RecoveryStatusReady)
	}
	return result
}

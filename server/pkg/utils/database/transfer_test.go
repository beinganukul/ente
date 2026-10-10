//go:build !js

package database

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTransferTypedValues(t *testing.T) {
	columns := []transferColumn{{Name: "blob", Kind: "binary"}, {Name: "id", Kind: "int"}, {Name: "null", Kind: "text"}, {Name: "flag", Kind: "bool"}, {Name: "array", Kind: "array"}, {Name: "json", Kind: "json"}}
	values := []any{[]byte{0, 255, 1}, int64(9007199254740993), nil, true, []byte(`{"b5","b6"}`), []byte(`{"large":9007199254740993,"null":null}`)}
	cells, err := transferCells(values, columns, Postgres)
	require.NoError(t, err)
	args, err := transferArgs(cells, columns, Turso)
	require.NoError(t, err)
	require.Equal(t, values[0], args[0])
	require.Equal(t, values[1], args[1])
	require.Nil(t, args[2])
	require.Equal(t, int64(1), args[3])
	require.Equal(t, `["b5","b6"]`, args[4])
	require.Equal(t, `{"large":9007199254740993,"null":null}`, args[5])
	back, err := transferCells(args, columns, Turso)
	require.NoError(t, err)
	require.Equal(t, cells, back)
}

// These explicit settings must reference separate disposable databases, never
// the application DB or other integration-test databases.
func TestTransferPostgresTursoRoundTrip(t *testing.T) {
	sourceDSN, targetDSN, tursoURL := os.Getenv("ENTE_TRANSFER_TEST_PG_SOURCE"), os.Getenv("ENTE_TRANSFER_TEST_PG_TARGET"), os.Getenv("ENTE_TRANSFER_TEST_TURSO_URL")
	if sourceDSN == "" || targetDSN == "" || tursoURL == "" {
		t.Skip("set three explicit ENTE_TRANSFER_TEST_* disposable databases")
	}
	require.Contains(t, sourceDSN, "dbname=ente_transfer_source")
	require.Contains(t, targetDSN, "dbname=ente_transfer_target")
	require.Contains(t, tursoURL, "127.0.0.1")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	source := Config{Provider: Postgres, PostgresDSN: sourceDSN}
	target := Config{Provider: Postgres, PostgresDSN: targetDSN}
	turso := Config{Provider: Turso, TursoURL: tursoURL}
	for _, cfg := range []Config{source, target, turso} {
		require.NoError(t, Migrate(ctx, cfg, "../../../migrations"))
	}
	db, err := Open(ctx, source)
	require.NoError(t, err)
	defer db.Close()
	var tables int
	require.NoError(t, db.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='public'").Scan(&tables))
	require.Greater(t, tables, 60, "full Museum migration expected")
	const userID int64 = 9007199254740993
	_, err = db.ExecContext(ctx, `INSERT INTO users(user_id,creation_time,encrypted_email,email_decryption_nonce,email_hash,delete_feedback) OVERRIDING SYSTEM VALUE VALUES ($1,42,$2,$3,'transfer-test',$4) ON CONFLICT DO NOTHING`, userID, []byte{0, 255, 1}, []byte{0, 1}, `{"large":9007199254740993,"null":null,"array":[true,"x"]}`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO collections(collection_id,owner_id,encrypted_key,key_decryption_nonce,type,attributes,updation_time) OVERRIDING SYSTEM VALUE VALUES ($1,$2,'encrypted','nonce','album','{"version":1}',42) ON CONFLICT DO NOTHING`, userID+1, userID)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO tokens(user_id,token,token_hash,creation_time,last_used_at,app) VALUES ($1,NULL,$2,42,43,'auth') ON CONFLICT DO NOTHING`, userID, bytes.Repeat([]byte{255}, 32))
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO file_data(file_id,user_id,data_type,size,latest_bucket,replicated_buckets,created_at,updated_at) VALUES ($1,$2,'img_preview',123,'b2-eu-cen',ARRAY['b5','b6']::s3region[],42,43) ON CONFLICT DO NOTHING`, userID+2, userID)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO file_data(file_id,user_id,data_type,size,latest_bucket,replicated_buckets,created_at,updated_at) VALUES ($1,$2,'img_preview',123,'b2-eu-cen',ARRAY[NULL]::s3region[],42,43) ON CONFLICT DO NOTHING`, userID+3, userID)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO comments(id,collection_id,parent_comment_id,user_id,cipher,nonce,created_at,updated_at) VALUES ('z-parent',$1,NULL,$2,'cipher','nonce',42,43),('a-child',$1,'z-parent',$2,'cipher-child','nonce',44,45) ON CONFLICT DO NOTHING`, userID+1, userID)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO reactions(id,collection_id,comment_id,user_id,cipher,nonce,created_at,updated_at) VALUES ('reaction',$1,'a-child',$2,'reaction-cipher','nonce',46,47) ON CONFLICT DO NOTHING`, userID+1, userID)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO subscription_logs(user_id,payment_provider,notification,verification_response,created_at) SELECT $1,'TRANSFER_TEST',jsonb_build_object('index',n,'large',9007199254740993),'null'::jsonb,'2026-01-02 03:04:05.123456'::timestamp FROM generate_series(1,300) n WHERE NOT EXISTS (SELECT 1 FROM subscription_logs WHERE payment_provider='TRANSFER_TEST')`, userID)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, "SELECT setval(pg_get_serial_sequence('users','user_id'),$1,true)", userID+99)
	require.NoError(t, err)
	directory := t.TempDir()
	pgFile := filepath.Join(directory, "postgres.jsonl")
	sqliteFile := filepath.Join(directory, "turso.jsonl")
	require.NoError(t, ExportSnapshot(ctx, source, pgFile))
	info, err := os.Stat(pgFile)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0600), info.Mode().Perm())
	require.Error(t, ExportSnapshot(ctx, source, pgFile), "must not overwrite backups")
	require.NoError(t, ImportSnapshot(ctx, turso, pgFile))
	require.ErrorContains(t, ImportSnapshot(ctx, turso, pgFile), "not empty")
	require.NoError(t, ExportSnapshot(ctx, turso, sqliteFile))
	// The footer is a commit marker. Truncation must leave the target untouched.
	contents, err := os.ReadFile(sqliteFile)
	require.NoError(t, err)
	lines := bytes.Split(bytes.TrimSpace(contents), []byte{'\n'})
	truncated := filepath.Join(directory, "truncated.jsonl")
	require.NoError(t, os.WriteFile(truncated, bytes.Join(lines[:len(lines)-1], []byte{'\n'}), 0600))
	require.ErrorContains(t, ImportSnapshot(ctx, target, truncated), "truncated")
	targetDB, err := Open(ctx, target)
	require.NoError(t, err)
	defer targetDB.Close()
	var count int
	require.NoError(t, targetDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&count))
	require.Zero(t, count)
	require.NoError(t, ImportSnapshot(ctx, target, sqliteFile))
	var blob, hash []byte
	var id int64
	var token *string
	var array string
	require.NoError(t, targetDB.QueryRowContext(ctx, "SELECT user_id,encrypted_email FROM users").Scan(&id, &blob))
	require.Equal(t, userID, id)
	require.Equal(t, []byte{0, 255, 1}, blob)
	require.NoError(t, targetDB.QueryRowContext(ctx, "SELECT token,token_hash FROM tokens").Scan(&token, &hash))
	require.Nil(t, token)
	require.Equal(t, bytes.Repeat([]byte{255}, 32), hash)
	require.NoError(t, targetDB.QueryRowContext(ctx, "SELECT replicated_buckets::text FROM file_data WHERE file_id=$1", userID+2).Scan(&array))
	require.Equal(t, "{b5,b6}", array)
	require.NoError(t, targetDB.QueryRowContext(ctx, "SELECT replicated_buckets::text FROM file_data WHERE file_id=$1", userID+3).Scan(&array))
	require.Equal(t, "{NULL}", array, "an enum-array NULL element differs from a SQL NULL array")
	var next int64
	require.NoError(t, targetDB.QueryRowContext(ctx, "SELECT nextval(pg_get_serial_sequence('users','user_id'))").Scan(&next))
	require.Equal(t, userID+100, next)
	var timestamp time.Time
	require.NoError(t, targetDB.QueryRowContext(ctx, "SELECT created_at FROM subscription_logs LIMIT 1").Scan(&timestamp))
	require.Equal(t, "2026-01-02 03:04:05.123456", timestamp.Format("2006-01-02 15:04:05.999999"))
	require.NoError(t, targetDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM subscription_logs").Scan(&count))
	require.Equal(t, 300, count, "pagination must retain every row")
	var sourceCount int
	require.NoError(t, db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&sourceCount))
	require.Equal(t, 1, sourceCount, "source must be retained")
}

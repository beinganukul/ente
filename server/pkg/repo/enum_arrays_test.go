package repo

import (
	"testing"

	"github.com/ente/museum/pkg/utils/database"
	"github.com/stretchr/testify/require"
)

func TestRegionArrayConstraintsAcrossProviders(t *testing.T) {
	_, db, ownerID := setupCollectionMembershipTest(t)
	fileID := insertObjectTestFile(t, db, ownerID)
	for _, table := range []string{"file_data", "user_attachments", "object_keys", "embeddings"} {
		t.Run(table, func(t *testing.T) {
			for _, tc := range []struct {
				name, pg, sqlite string
				valid            bool
			}{
				{"valid", "{b5}", `["b5"]`, true},
				{"null", "{NULL}", `[null]`, true},
				{"invalid", "{invalid-region}", `["invalid-region"]`, false},
				{"numeric", "{42}", `[42]`, false},
				{"duplicate", "{b5,b5}", `["b5","b5"]`, table == "object_keys" || table == "embeddings"},
				{"duplicate_null", "{NULL,NULL}", `[null,null]`, table == "object_keys" || table == "embeddings"},
			} {
				t.Run(tc.name, func(t *testing.T) {
					_, err := db.Exec("DELETE FROM " + table)
					require.NoError(t, err)
					array := database.SQL(db, tc.pg, tc.sqlite)
					placeholder := database.SQL(db, "$3::s3region[]", "$3")
					var query string
					switch table {
					case "file_data":
						query = "INSERT INTO file_data(file_id,user_id,data_type,size,latest_bucket,replicated_buckets) VALUES($1,$2,'img_preview',1,'b2-eu-cen'," + placeholder + ")"
					case "user_attachments":
						query = "INSERT INTO user_attachments(attachment_id,user_id,attachment_type,size,latest_bucket,replicated_buckets) VALUES(CAST($1 AS TEXT),$2,'profile_picture',1,'b2-eu-cen'," + placeholder + ")"
					case "object_keys":
						query = "INSERT INTO object_keys(file_id,o_type,object_key,size,datacenters) SELECT $1,'file',CAST($2 AS TEXT),1," + placeholder
					case "embeddings":
						query = "INSERT INTO embeddings(file_id,owner_id,model,datacenters) VALUES($1,$2,'ggml-clip'," + placeholder + ")"
					}
					_, err = db.Exec(query, fileID, ownerID, array)
					if tc.valid {
						require.NoError(t, err)
						column := "datacenters"
						if table == "file_data" || table == "user_attachments" {
							column = "replicated_buckets"
						}
						_, err = db.Exec("UPDATE "+table+" SET "+column+"="+database.SQL(db, "$1::s3region[]", "$1"), database.SQL(db, "{invalid-region}", `["invalid-region"]`))
						require.Error(t, err, "updates must validate enum elements")
					} else {
						require.Error(t, err)
					}
				})
			}
		})
	}
}

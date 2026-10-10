-- Museum schema at PostgreSQL migration 151. Arrays are JSON; timestamps remain UTC microseconds.

CREATE TABLE "anon_users" (
    "id" TEXT NOT NULL,
    "collection_id" BIGINT NOT NULL CHECK ("collection_id" IS NULL OR (typeof("collection_id") = 'integer')),
    "cipher" TEXT NOT NULL,
    "nonce" TEXT NOT NULL,
    "created_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("created_at" IS NULL OR (typeof("created_at") = 'integer')),
    "updated_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("updated_at" IS NULL OR (typeof("updated_at") = 'integer')),
    CONSTRAINT "anon_users_pkey" PRIMARY KEY (id)
);

CREATE INDEX idx_anon_users_collection ON anon_users (collection_id);

CREATE TABLE "authenticator_entity" (
    "id" TEXT NOT NULL,
    "user_id" BIGINT NOT NULL CHECK ("user_id" IS NULL OR (typeof("user_id") = 'integer')),
    "encrypted_data" TEXT,
    "header" TEXT,
    "created_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("created_at" IS NULL OR (typeof("created_at") = 'integer')),
    "updated_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("updated_at" IS NULL OR (typeof("updated_at") = 'integer')),
    "is_deleted" BIGINT DEFAULT (false) CHECK ("is_deleted" IS NULL OR ("is_deleted" IN (0, 1))),
    CONSTRAINT "authenticator_entity_pkey" PRIMARY KEY (id),
    CONSTRAINT "authenticator_entity_state_constraint" CHECK ((((is_deleted IS TRUE) AND (encrypted_data IS NULL)) OR ((is_deleted IS FALSE) AND (encrypted_data IS NOT NULL)))),
    CONSTRAINT "fk_authenticator_key_user_id" FOREIGN KEY (user_id) REFERENCES authenticator_key(user_id) ON DELETE CASCADE
);

CREATE INDEX authenticator_entity_updated_at_time_index ON authenticator_entity (user_id, updated_at);

CREATE TABLE "authenticator_key" (
    "user_id" BIGINT NOT NULL CHECK ("user_id" IS NULL OR (typeof("user_id") = 'integer')),
    "encrypted_key" TEXT NOT NULL,
    "header" TEXT NOT NULL,
    "created_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("created_at" IS NULL OR (typeof("created_at") = 'integer')),
    "updated_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("updated_at" IS NULL OR (typeof("updated_at") = 'integer')),
    CONSTRAINT "authenticator_key_pkey" PRIMARY KEY (user_id),
    CONSTRAINT "fk_authenticator_key_user_id" FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE
);

CREATE TABLE "casting" (
    "id" TEXT NOT NULL,
    "code" TEXT NOT NULL CHECK ("code" IS NULL OR (length("code") <= 16)),
    "public_key" TEXT NOT NULL CHECK ("public_key" IS NULL OR (length("public_key") <= 512)),
    "collection_id" BIGINT CHECK ("collection_id" IS NULL OR (typeof("collection_id") = 'integer')),
    "cast_user" BIGINT CHECK ("cast_user" IS NULL OR (typeof("cast_user") = 'integer')),
    "encrypted_payload" TEXT,
    "token" TEXT CHECK ("token" IS NULL OR (length("token") <= 512)),
    "is_deleted" BIGINT DEFAULT (false) CHECK ("is_deleted" IS NULL OR ("is_deleted" IN (0, 1))),
    "created_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("created_at" IS NULL OR (typeof("created_at") = 'integer')),
    "last_used_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("last_used_at" IS NULL OR (typeof("last_used_at") = 'integer')),
    "ip" TEXT NOT NULL,
    "device_name" TEXT,
    "pq_public_key" TEXT,
    CONSTRAINT "casting_pkey" PRIMARY KEY (id)
);

CREATE UNIQUE INDEX casting_code_unique_idx ON casting (code) WHERE (is_deleted = false);

CREATE TABLE "collection_actions" (
    "id" TEXT NOT NULL,
    "user_id" BIGINT NOT NULL CHECK ("user_id" IS NULL OR (typeof("user_id") = 'integer')),
    "actor_user_id" BIGINT NOT NULL CHECK ("actor_user_id" IS NULL OR (typeof("actor_user_id") = 'integer')),
    "collection_id" BIGINT NOT NULL CHECK ("collection_id" IS NULL OR (typeof("collection_id") = 'integer')),
    "file_id" BIGINT CHECK ("file_id" IS NULL OR (typeof("file_id") = 'integer')),
    "data" ANY CHECK ("data" IS NULL OR (json_valid("data"))),
    "action" TEXT NOT NULL,
    "is_pending" BIGINT NOT NULL DEFAULT (true) CHECK ("is_pending" IS NULL OR ("is_pending" IN (0, 1))),
    "created_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("created_at" IS NULL OR (typeof("created_at") = 'integer')),
    "updated_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("updated_at" IS NULL OR (typeof("updated_at") = 'integer')),
    CONSTRAINT "collection_actions_file_required" CHECK ((((action IN ('REMOVE', 'DELETE_SUGGESTED')) AND (file_id IS NOT NULL)) OR (action NOT IN ('REMOVE', 'DELETE_SUGGESTED')))),
    CONSTRAINT "collection_actions_pkey" PRIMARY KEY (id)
);

CREATE INDEX collection_actions_user_id_idx ON collection_actions (user_id);

CREATE INDEX collection_actions_collection_id_idx ON collection_actions (collection_id);

CREATE INDEX collection_actions_pending_remove_delete_user_time_idx ON collection_actions (user_id, updated_at) WHERE ((action IN ('REMOVE', 'DELETE_SUGGESTED')) AND (is_pending = true));

CREATE TABLE "collection_files" (
    "file_id" BIGINT NOT NULL CHECK ("file_id" IS NULL OR (typeof("file_id") = 'integer')),
    "collection_id" BIGINT NOT NULL CHECK ("collection_id" IS NULL OR (typeof("collection_id") = 'integer')),
    "encrypted_key" TEXT NOT NULL,
    "key_decryption_nonce" TEXT NOT NULL,
    "is_deleted" BIGINT DEFAULT (false) CHECK ("is_deleted" IS NULL OR ("is_deleted" IN (0, 1))),
    "updation_time" BIGINT NOT NULL CHECK ("updation_time" IS NULL OR (typeof("updation_time") = 'integer')),
    "c_owner_id" BIGINT CHECK ("c_owner_id" IS NULL OR (typeof("c_owner_id") = 'integer')),
    "f_owner_id" BIGINT CHECK ("f_owner_id" IS NULL OR (typeof("f_owner_id") = 'integer')),
    "created_at" BIGINT DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("created_at" IS NULL OR (typeof("created_at") = 'integer')),
    "action_user" BIGINT CHECK ("action_user" IS NULL OR (typeof("action_user") = 'integer')),
    "action" TEXT,
    CONSTRAINT "fk_collection_files_collection_id" FOREIGN KEY (collection_id) REFERENCES collections(collection_id) ON DELETE CASCADE,
    CONSTRAINT "fk_collection_files_file_id" FOREIGN KEY (file_id) REFERENCES files(file_id) ON DELETE CASCADE,
    CONSTRAINT "unique_collection_files_cid_fid" UNIQUE (collection_id, file_id)
);

CREATE INDEX collection_files_collection_id_index ON collection_files (collection_id);

CREATE INDEX collection_files_file_id_index ON collection_files (file_id);

CREATE TABLE "collection_shares" (
    "collection_id" BIGINT NOT NULL CHECK ("collection_id" IS NULL OR (typeof("collection_id") = 'integer')),
    "from_user_id" BIGINT NOT NULL CHECK ("from_user_id" IS NULL OR (typeof("from_user_id") = 'integer')),
    "to_user_id" BIGINT NOT NULL CHECK ("to_user_id" IS NULL OR (typeof("to_user_id") = 'integer')),
    "encrypted_key" TEXT NOT NULL,
    "updation_time" BIGINT NOT NULL CHECK ("updation_time" IS NULL OR (typeof("updation_time") = 'integer')),
    "is_deleted" BIGINT DEFAULT (false) CHECK ("is_deleted" IS NULL OR ("is_deleted" IN (0, 1))),
    "role_type" TEXT DEFAULT ('VIEWER') CHECK ("role_type" IS NULL OR ("role_type" IN ('VIEWER', 'COLLABORATOR', 'OWNER', 'ADMIN'))),
    "magic_metadata" ANY CHECK ("magic_metadata" IS NULL OR (json_valid("magic_metadata"))),
    "shared_at" BIGINT CHECK ("shared_at" IS NULL OR (typeof("shared_at") = 'integer')),
    CONSTRAINT "collection_shares_collection_id_from_user_id_to_user_id_key" UNIQUE (collection_id, from_user_id, to_user_id),
    CONSTRAINT "fk_collection_shares_collection_id" FOREIGN KEY (collection_id) REFERENCES collections(collection_id) ON DELETE CASCADE,
    CONSTRAINT "fk_collection_shares_from_user_id" FOREIGN KEY (from_user_id) REFERENCES users(user_id) ON DELETE CASCADE,
    CONSTRAINT "fk_collection_shares_to_user_id" FOREIGN KEY (to_user_id) REFERENCES users(user_id) ON DELETE CASCADE
);

CREATE INDEX collection_shares_to_user_id_index ON collection_shares (to_user_id);

CREATE TABLE "collections" (
    "collection_id" INTEGER PRIMARY KEY AUTOINCREMENT,
    "owner_id" BIGINT NOT NULL CHECK ("owner_id" IS NULL OR (typeof("owner_id") = 'integer')),
    "encrypted_key" TEXT NOT NULL,
    "key_decryption_nonce" TEXT NOT NULL,
    "name" TEXT,
    "type" TEXT NOT NULL,
    "attributes" ANY NOT NULL CHECK ("attributes" IS NULL OR (json_valid("attributes"))),
    "updation_time" BIGINT NOT NULL CHECK ("updation_time" IS NULL OR (typeof("updation_time") = 'integer')),
    "is_deleted" BIGINT DEFAULT (false) CHECK ("is_deleted" IS NULL OR ("is_deleted" IN (0, 1))),
    "encrypted_name" TEXT,
    "name_decryption_nonce" TEXT,
    "magic_metadata" ANY CHECK ("magic_metadata" IS NULL OR (json_valid("magic_metadata"))),
    "pub_magic_metadata" ANY CHECK ("pub_magic_metadata" IS NULL OR (json_valid("pub_magic_metadata"))),
    "app" TEXT NOT NULL DEFAULT ('photos') CHECK ("app" IS NULL OR ("app" IN ('photos', 'auth', 'locker'))),
    CONSTRAINT "fk_collections_owner_id" FOREIGN KEY (owner_id) REFERENCES users(user_id) ON DELETE CASCADE
);

CREATE INDEX collections_owner_id_index ON collections (owner_id);

CREATE UNIQUE INDEX collections_uncategorized_constraint_index_v2 ON collections (owner_id, app) WHERE (type = 'uncategorized');

CREATE UNIQUE INDEX collections_favorites_constraint_index_v2 ON collections (owner_id, app) WHERE (type = 'favorites');

CREATE TABLE "comments" (
    "id" TEXT NOT NULL,
    "collection_id" BIGINT NOT NULL CHECK ("collection_id" IS NULL OR (typeof("collection_id") = 'integer')),
    "file_id" BIGINT CHECK ("file_id" IS NULL OR (typeof("file_id") = 'integer')),
    "parent_comment_id" TEXT,
    "user_id" BIGINT NOT NULL CHECK ("user_id" IS NULL OR (typeof("user_id") = 'integer')),
    "anon_user_id" TEXT,
    "cipher" TEXT,
    "nonce" TEXT,
    "is_deleted" BIGINT NOT NULL DEFAULT (false) CHECK ("is_deleted" IS NULL OR ("is_deleted" IN (0, 1))),
    "created_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("created_at" IS NULL OR (typeof("created_at") = 'integer')),
    "updated_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("updated_at" IS NULL OR (typeof("updated_at") = 'integer')),
    CONSTRAINT "comments_cipher_on_delete" CHECK ((is_deleted = false AND cipher IS NOT NULL AND nonce IS NOT NULL) OR is_deleted = true),
    CONSTRAINT "comments_pkey" PRIMARY KEY (id),
    CONSTRAINT "comments_user_or_anon_chk" CHECK ((((user_id = '-1') AND (anon_user_id IS NOT NULL)) OR ((user_id > 0) AND (anon_user_id IS NULL)))),
    CONSTRAINT "fk_comments_anon_user" FOREIGN KEY (anon_user_id) REFERENCES anon_users(id)
);

CREATE INDEX idx_comments_scope ON comments (collection_id, file_id) WHERE (is_deleted = false);

CREATE INDEX idx_comments_parent ON comments (parent_comment_id) WHERE (is_deleted = false);

CREATE INDEX idx_comments_user ON comments (user_id) WHERE (is_deleted = false);

CREATE INDEX idx_comments_anon ON comments (anon_user_id) WHERE (is_deleted = false);

CREATE INDEX idx_comments_collection_updated_at ON comments (collection_id, updated_at);

CREATE TABLE "contact_entity" (
    "id" TEXT NOT NULL,
    "user_id" BIGINT NOT NULL CHECK ("user_id" IS NULL OR (typeof("user_id") = 'integer')),
    "contact_user_id" BIGINT NOT NULL CHECK ("contact_user_id" IS NULL OR (typeof("contact_user_id") = 'integer')),
    "profile_picture_attachment_id" TEXT,
    "encrypted_key" BLOB NOT NULL CHECK ("encrypted_key" IS NULL OR (typeof("encrypted_key") = 'blob')),
    "encrypted_data" BLOB CHECK ("encrypted_data" IS NULL OR (typeof("encrypted_data") = 'blob')),
    "is_deleted" BIGINT NOT NULL DEFAULT (false) CHECK ("is_deleted" IS NULL OR ("is_deleted" IN (0, 1))),
    "created_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("created_at" IS NULL OR (typeof("created_at") = 'integer')),
    "updated_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("updated_at" IS NULL OR (typeof("updated_at") = 'integer')),
    CONSTRAINT "contact_entity_pkey" PRIMARY KEY (id),
    CONSTRAINT "contact_entity_state_constraint" CHECK ((((is_deleted = false) AND (encrypted_data IS NOT NULL)) OR ((is_deleted = true) AND (encrypted_data IS NULL) AND (profile_picture_attachment_id IS NULL)))),
    CONSTRAINT "fk_contact_entity_user_id" FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE
);

CREATE INDEX idx_contact_entity_user_updated_at ON contact_entity (user_id, updated_at);

CREATE UNIQUE INDEX idx_contact_entity_user_contact_user_id ON contact_entity (user_id, contact_user_id);

CREATE TABLE "data_cleanup" (
    "user_id" BIGINT NOT NULL CHECK ("user_id" IS NULL OR (typeof("user_id") = 'integer')),
    "stage" TEXT NOT NULL DEFAULT ('scheduled') CHECK ("stage" IS NULL OR ("stage" IN ('scheduled', 'collection', 'trash', 'storage', 'completed'))),
    "stage_schedule_time" BIGINT NOT NULL DEFAULT ((CAST(unixepoch('subsec') * 1000000 AS INTEGER) + (((((7 * (24)) * 60) * 60) * 1000) * 1000))) CHECK ("stage_schedule_time" IS NULL OR (typeof("stage_schedule_time") = 'integer')),
    "stage_attempt_count" BIGINT NOT NULL DEFAULT (0) CHECK ("stage_attempt_count" IS NULL OR (typeof("stage_attempt_count") = 'integer' AND "stage_attempt_count" BETWEEN -2147483648 AND 2147483647)),
    "status" TEXT NOT NULL DEFAULT (''),
    "created_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("created_at" IS NULL OR (typeof("created_at") = 'integer')),
    "updated_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("updated_at" IS NULL OR (typeof("updated_at") = 'integer')),
    "email_hash" TEXT,
    CONSTRAINT "data_cleanup_pkey" PRIMARY KEY (user_id)
);

CREATE INDEX idx_data_cleanup_scheduled_email_hash ON data_cleanup (email_hash, created_at DESC) WHERE ((stage = 'scheduled') AND (email_hash IS NOT NULL));

CREATE TABLE "discount_coupons" (
    "provider_name" TEXT NOT NULL,
    "code" TEXT NOT NULL,
    "claimed_by_user_id" BIGINT CHECK ("claimed_by_user_id" IS NULL OR (typeof("claimed_by_user_id") = 'integer')),
    "claimed_at" BIGINT CHECK ("claimed_at" IS NULL OR (typeof("claimed_at") = 'integer')),
    "sent_count" BIGINT DEFAULT (0) CHECK ("sent_count" IS NULL OR (typeof("sent_count") = 'integer' AND "sent_count" BETWEEN -2147483648 AND 2147483647)),
    "created_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("created_at" IS NULL OR (typeof("created_at") = 'integer')),
    "updated_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("updated_at" IS NULL OR (typeof("updated_at") = 'integer')),
    CONSTRAINT "discount_coupons_provider_code_unique" UNIQUE (provider_name, code)
);

CREATE UNIQUE INDEX discount_coupons_provider_user_unique ON discount_coupons (provider_name, claimed_by_user_id) WHERE (claimed_by_user_id IS NOT NULL);

CREATE TABLE "embeddings" (
    "file_id" BIGINT NOT NULL CHECK ("file_id" IS NULL OR (typeof("file_id") = 'integer')),
    "owner_id" BIGINT NOT NULL CHECK ("owner_id" IS NULL OR (typeof("owner_id") = 'integer')),
    "model" TEXT NOT NULL CHECK ("model" IS NULL OR ("model" IN ('ggml-clip', 'onnx-clip', 'file-ml-clip-face', 'derived'))),
    "encrypted_embedding" TEXT,
    "decryption_header" TEXT,
    "updated_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("updated_at" IS NULL OR (typeof("updated_at") = 'integer')),
    "size" BIGINT CHECK ("size" IS NULL OR (typeof("size") = 'integer' AND "size" BETWEEN -2147483648 AND 2147483647)),
    "version" BIGINT DEFAULT (1) CHECK ("version" IS NULL OR (typeof("version") = 'integer' AND "version" BETWEEN -2147483648 AND 2147483647)),
    "datacenters" TEXT DEFAULT ('["b2-eu-cen"]') CHECK ("datacenters" IS NULL OR (json_valid("datacenters") AND json_type("datacenters") = 'array')),
    CONSTRAINT "fk_embeddings_file_id" FOREIGN KEY (file_id) REFERENCES files(file_id) ON DELETE CASCADE,
    CONSTRAINT "unique_embeddings_file_id_model" UNIQUE (file_id, model)
);

CREATE INDEX embeddings_owner_id_updated_at_index ON embeddings (owner_id, updated_at);

CREATE TABLE "emergency_contact" (
    "user_id" BIGINT NOT NULL CHECK ("user_id" IS NULL OR (typeof("user_id") = 'integer')),
    "emergency_contact_id" BIGINT NOT NULL CHECK ("emergency_contact_id" IS NULL OR (typeof("emergency_contact_id") = 'integer')),
    "state" TEXT NOT NULL,
    "created_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("created_at" IS NULL OR (typeof("created_at") = 'integer')),
    "updated_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("updated_at" IS NULL OR (typeof("updated_at") = 'integer')),
    "notice_period_in_hrs" BIGINT NOT NULL CHECK ("notice_period_in_hrs" IS NULL OR (typeof("notice_period_in_hrs") = 'integer' AND "notice_period_in_hrs" BETWEEN -2147483648 AND 2147483647)),
    "encrypted_key" TEXT,
    CONSTRAINT "chk_encrypted_key_null" CHECK ((((state IN ('REVOKED', 'DELETED', 'CONTACT_LEFT', 'CONTACT_DENIED')) AND (encrypted_key IS NULL)) OR ((state NOT IN ('REVOKED', 'DELETED', 'CONTACT_LEFT', 'CONTACT_DENIED')) AND (encrypted_key IS NOT NULL)))),
    CONSTRAINT "chk_user_id_not_equal_emergency_contact_id" CHECK ((user_id <> emergency_contact_id)),
    CONSTRAINT "emergency_contact_state_check" CHECK ((state IN ('INVITED', 'ACCEPTED', 'REVOKED', 'DELETED', 'CONTACT_LEFT', 'CONTACT_DENIED'))),
    CONSTRAINT "fk_emergency_contact_emergency_contact_id" FOREIGN KEY (emergency_contact_id) REFERENCES users(user_id) ON DELETE CASCADE,
    CONSTRAINT "fk_emergency_contact_user_id" FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE,
    CONSTRAINT "unique_user_emergency_contact" UNIQUE (user_id, emergency_contact_id)
);

CREATE INDEX idx_emergency_contact_id ON emergency_contact (emergency_contact_id);

CREATE TABLE "emergency_recovery" (
    "id" TEXT NOT NULL,
    "user_id" BIGINT NOT NULL CHECK ("user_id" IS NULL OR (typeof("user_id") = 'integer')),
    "emergency_contact_id" BIGINT NOT NULL CHECK ("emergency_contact_id" IS NULL OR (typeof("emergency_contact_id") = 'integer')),
    "status" TEXT NOT NULL,
    "wait_till" BIGINT CHECK ("wait_till" IS NULL OR (typeof("wait_till") = 'integer')),
    "next_reminder_at" BIGINT CHECK ("next_reminder_at" IS NULL OR (typeof("next_reminder_at") = 'integer')),
    "created_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("created_at" IS NULL OR (typeof("created_at") = 'integer')),
    "updated_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("updated_at" IS NULL OR (typeof("updated_at") = 'integer')),
    CONSTRAINT "emergency_recovery_pkey" PRIMARY KEY (id),
    CONSTRAINT "emergency_recovery_status_check" CHECK ((status IN ('WAITING', 'REJECTED', 'RECOVERED', 'STOPPED', 'READY'))),
    CONSTRAINT "fk_emergency_recovery_emergency_contact_id" FOREIGN KEY (emergency_contact_id) REFERENCES users(user_id) ON DELETE CASCADE,
    CONSTRAINT "fk_emergency_recovery_user_id" FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX idx_emergency_recovery_limit_active_recovery ON emergency_recovery (user_id, emergency_contact_id, status) WHERE (status IN ('WAITING', 'READY'));

CREATE INDEX idx_emergency_recovery_user_id ON emergency_recovery (user_id);

CREATE INDEX idx_emergency_recovery_next_reminder_at ON emergency_recovery (next_reminder_at);

CREATE TABLE "entity_data" (
    "user_id" BIGINT NOT NULL CHECK ("user_id" IS NULL OR (typeof("user_id") = 'integer')),
    "type" TEXT NOT NULL,
    "encrypted_data" TEXT,
    "header" TEXT,
    "created_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("created_at" IS NULL OR (typeof("created_at") = 'integer')),
    "updated_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("updated_at" IS NULL OR (typeof("updated_at") = 'integer')),
    "is_deleted" BIGINT DEFAULT (false) CHECK ("is_deleted" IS NULL OR ("is_deleted" IN (0, 1))),
    "id" TEXT NOT NULL,
    CONSTRAINT "entity_data_pkey" PRIMARY KEY (id),
    CONSTRAINT "entity_data_state_constraint" CHECK ((((is_deleted IS TRUE) AND (encrypted_data IS NULL)) OR ((is_deleted IS FALSE) AND (encrypted_data IS NOT NULL)))),
    CONSTRAINT "fk_entity_key_user_id_and_type" FOREIGN KEY (user_id, type) REFERENCES entity_key(user_id, type) ON DELETE CASCADE
);

CREATE INDEX entity_data_updated_at_time_index ON entity_data (user_id, updated_at);

CREATE TABLE "entity_key" (
    "user_id" BIGINT NOT NULL CHECK ("user_id" IS NULL OR (typeof("user_id") = 'integer')),
    "type" TEXT NOT NULL,
    "encrypted_key" TEXT NOT NULL,
    "header" TEXT NOT NULL,
    "created_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("created_at" IS NULL OR (typeof("created_at") = 'integer')),
    "updated_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("updated_at" IS NULL OR (typeof("updated_at") = 'integer')),
    CONSTRAINT "entity_key_pkey" PRIMARY KEY (user_id, type),
    CONSTRAINT "fk_entity_key_user_id" FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE
);

CREATE TABLE "events" (
    "id" TEXT NOT NULL,
    "event" TEXT NOT NULL,
    "app" TEXT NOT NULL,
    "platform" TEXT NOT NULL,
    "data" ANY NOT NULL DEFAULT ('{}') CHECK ("data" IS NULL OR (json_valid("data"))),
    "user_id" BIGINT CHECK ("user_id" IS NULL OR (typeof("user_id") = 'integer')),
    "created_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("created_at" IS NULL OR (typeof("created_at") = 'integer')),
    "updated_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("updated_at" IS NULL OR (typeof("updated_at") = 'integer')),
    CONSTRAINT "events_data_check" CHECK ((json_type(data) = 'object'))
);

CREATE UNIQUE INDEX events_id_event_idx ON events (id, event);

CREATE INDEX events_user_id_idx ON events (user_id);

CREATE TABLE "families" (
    "id" TEXT NOT NULL,
    "admin_id" BIGINT NOT NULL CHECK ("admin_id" IS NULL OR (typeof("admin_id") = 'integer')),
    "member_id" BIGINT NOT NULL CHECK ("member_id" IS NULL OR (typeof("member_id") = 'integer')),
    "status" TEXT NOT NULL,
    "token" TEXT,
    "percentage" BIGINT NOT NULL DEFAULT ('-1') CHECK ("percentage" IS NULL OR (typeof("percentage") = 'integer' AND "percentage" BETWEEN -2147483648 AND 2147483647)),
    "created_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("created_at" IS NULL OR (typeof("created_at") = 'integer')),
    "updated_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("updated_at" IS NULL OR (typeof("updated_at") = 'integer')),
    "storage_limit" BIGINT CHECK ("storage_limit" IS NULL OR (typeof("storage_limit") = 'integer')),
    CONSTRAINT "families_member_state_constraint" CHECK ((((admin_id <> member_id) AND (status NOT IN ('SELF', 'CLOSED'))) OR ((admin_id = member_id) AND (status IN ('SELF', 'CLOSED'))))),
    CONSTRAINT "families_pkey" PRIMARY KEY (id),
    CONSTRAINT "families_status_check" CHECK ((status IN ('SELF', 'CLOSED', 'INVITED', 'ACCEPTED', 'DECLINED', 'REVOKED', 'REMOVED', 'LEFT'))),
    CONSTRAINT "families_token_key" UNIQUE (token),
    CONSTRAINT "fk_family_admin_id" FOREIGN KEY (admin_id) REFERENCES users(user_id) ON DELETE CASCADE
);

CREATE INDEX fk_families_admin_id ON families (admin_id);

CREATE UNIQUE INDEX uidx_one_family_check ON families (member_id, status) WHERE (status IN ('ACCEPTED', 'SELF'));

CREATE UNIQUE INDEX uidx_families_member_mapping ON families (admin_id, member_id);

CREATE TABLE "file_data" (
    "file_id" BIGINT NOT NULL CHECK ("file_id" IS NULL OR (typeof("file_id") = 'integer')),
    "user_id" BIGINT NOT NULL CHECK ("user_id" IS NULL OR (typeof("user_id") = 'integer')),
    "data_type" TEXT NOT NULL CHECK ("data_type" IS NULL OR ("data_type" IN ('file', 'thumbnail', 'mldata', 'vid_preview', 'img_preview'))),
    "size" BIGINT NOT NULL CHECK ("size" IS NULL OR (typeof("size") = 'integer')),
    "latest_bucket" TEXT NOT NULL CHECK ("latest_bucket" IS NULL OR ("latest_bucket" IN ('b2-eu-cen', 'scw-eu-fr', 'scw-eu-fr-locked', 'wasabi-eu-central-2', 'wasabi-eu-central-2-v3', 'scw-eu-fr-v3', 'wasabi-eu-central-2-derived', 'b5', 'b6'))),
    "replicated_buckets" TEXT NOT NULL DEFAULT ('[]') CHECK ("replicated_buckets" IS NULL OR (json_valid("replicated_buckets") AND json_type("replicated_buckets") = 'array')),
    "delete_from_buckets" TEXT NOT NULL DEFAULT ('[]') CHECK ("delete_from_buckets" IS NULL OR (json_valid("delete_from_buckets") AND json_type("delete_from_buckets") = 'array')),
    "inflight_rep_buckets" TEXT NOT NULL DEFAULT ('[]') CHECK ("inflight_rep_buckets" IS NULL OR (json_valid("inflight_rep_buckets") AND json_type("inflight_rep_buckets") = 'array')),
    "is_deleted" BIGINT NOT NULL DEFAULT (false) CHECK ("is_deleted" IS NULL OR ("is_deleted" IN (0, 1))),
    "pending_sync" BIGINT NOT NULL DEFAULT (true) CHECK ("pending_sync" IS NULL OR ("pending_sync" IN (0, 1))),
    "sync_locked_till" BIGINT NOT NULL DEFAULT (0) CHECK ("sync_locked_till" IS NULL OR (typeof("sync_locked_till") = 'integer')),
    "created_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("created_at" IS NULL OR (typeof("created_at") = 'integer')),
    "updated_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("updated_at" IS NULL OR (typeof("updated_at") = 'integer')),
    "obj_id" TEXT,
    "obj_nonce" TEXT,
    "obj_size" BIGINT CHECK ("obj_size" IS NULL OR (typeof("obj_size") = 'integer')),
    CONSTRAINT "file_data_pkey" PRIMARY KEY (file_id, data_type)
);

CREATE INDEX idx_file_data_user_type_deleted ON file_data (user_id, data_type, is_deleted, size);

CREATE INDEX idx_file_data_pending_sync_locked_till ON file_data (is_deleted, sync_locked_till) WHERE (pending_sync = true);

CREATE INDEX idx_file_data_user_updated ON file_data (user_id, updated_at);

CREATE TABLE "files" (
    "file_id" INTEGER PRIMARY KEY AUTOINCREMENT,
    "owner_id" BIGINT NOT NULL CHECK ("owner_id" IS NULL OR (typeof("owner_id") = 'integer')),
    "file_decryption_header" TEXT NOT NULL,
    "thumbnail_decryption_header" TEXT NOT NULL,
    "metadata_decryption_header" TEXT NOT NULL,
    "encrypted_metadata" TEXT NOT NULL,
    "updation_time" BIGINT NOT NULL CHECK ("updation_time" IS NULL OR (typeof("updation_time") = 'integer')),
    "magic_metadata" ANY CHECK ("magic_metadata" IS NULL OR (json_valid("magic_metadata"))),
    "pub_magic_metadata" ANY CHECK ("pub_magic_metadata" IS NULL OR (json_valid("pub_magic_metadata"))),
    "info" ANY CHECK ("info" IS NULL OR (json_valid("info"))),
    "app" TEXT CHECK ("app" IS NULL OR ("app" IN ('photos', 'auth', 'locker'))),
    CONSTRAINT "files_app_supported" CHECK ((app IN ('photos', 'locker'))),
    CONSTRAINT "fk_files_owner_id" FOREIGN KEY (owner_id) REFERENCES users(user_id) ON DELETE CASCADE
);

CREATE INDEX files_updation_time_index ON files (updation_time);

CREATE INDEX files_owner_id_index ON files (owner_id);

CREATE TABLE "kex_store" (
    "id" TEXT NOT NULL,
    "wrapped_key" TEXT NOT NULL,
    "added_at" BIGINT NOT NULL CHECK ("added_at" IS NULL OR (typeof("added_at") = 'integer')),
    CONSTRAINT "kex_store_pkey" PRIMARY KEY (id)
);

CREATE TABLE "key_attributes" (
    "user_id" BIGINT NOT NULL CHECK ("user_id" IS NULL OR (typeof("user_id") = 'integer')),
    "kek_salt" TEXT NOT NULL,
    "kek_hash_bytes" BLOB CHECK ("kek_hash_bytes" IS NULL OR (typeof("kek_hash_bytes") = 'blob')),
    "encrypted_key" TEXT NOT NULL,
    "key_decryption_nonce" TEXT NOT NULL,
    "public_key" TEXT NOT NULL,
    "encrypted_secret_key" TEXT NOT NULL,
    "secret_key_decryption_nonce" TEXT NOT NULL,
    "mem_limit" BIGINT DEFAULT (67108864) CHECK ("mem_limit" IS NULL OR (typeof("mem_limit") = 'integer')),
    "ops_limit" BIGINT DEFAULT (2) CHECK ("ops_limit" IS NULL OR (typeof("ops_limit") = 'integer' AND "ops_limit" BETWEEN -2147483648 AND 2147483647)),
    "master_key_encrypted_with_recovery_key" TEXT,
    "master_key_decryption_nonce" TEXT,
    "recovery_key_encrypted_with_master_key" TEXT,
    "recovery_key_decryption_nonce" TEXT,
    "created_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("created_at" IS NULL OR (typeof("created_at") = 'integer')),
    CONSTRAINT "fk_key_attributes_user_id" FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE,
    CONSTRAINT "key_attributes_pkey" PRIMARY KEY (user_id)
);

CREATE TABLE "legacy_kit" (
    "id" TEXT NOT NULL,
    "user_id" BIGINT NOT NULL CHECK ("user_id" IS NULL OR (typeof("user_id") = 'integer')),
    "variant" BIGINT NOT NULL CHECK ("variant" IS NULL OR (typeof("variant") = 'integer' AND "variant" BETWEEN -2147483648 AND 2147483647)),
    "notice_period_in_hrs" BIGINT NOT NULL CHECK ("notice_period_in_hrs" IS NULL OR (typeof("notice_period_in_hrs") = 'integer' AND "notice_period_in_hrs" BETWEEN -2147483648 AND 2147483647)),
    "encrypted_recovery_blob" TEXT NOT NULL,
    "auth_public_key" TEXT NOT NULL,
    "encrypted_owner_blob" TEXT NOT NULL,
    "is_deleted" BIGINT NOT NULL DEFAULT (false) CHECK ("is_deleted" IS NULL OR ("is_deleted" IN (0, 1))),
    "created_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("created_at" IS NULL OR (typeof("created_at") = 'integer')),
    "updated_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("updated_at" IS NULL OR (typeof("updated_at") = 'integer')),
    "deleted_at" BIGINT CHECK ("deleted_at" IS NULL OR (typeof("deleted_at") = 'integer')),
    CONSTRAINT "legacy_kit_pkey" PRIMARY KEY (id),
    CONSTRAINT "legacy_kit_user_id_fkey" FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE,
    CONSTRAINT "legacy_kit_variant_check" CHECK ((variant = 1))
);

CREATE INDEX idx_legacy_kit_user_id ON legacy_kit (user_id);

CREATE INDEX idx_legacy_kit_active_user_id ON legacy_kit (user_id) WHERE (is_deleted = false);

CREATE TABLE "legacy_kit_challenge" (
    "id" TEXT NOT NULL,
    "kit_id" TEXT NOT NULL,
    "challenge_hash" TEXT NOT NULL,
    "expires_at" BIGINT NOT NULL CHECK ("expires_at" IS NULL OR (typeof("expires_at") = 'integer')),
    "created_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("created_at" IS NULL OR (typeof("created_at") = 'integer')),
    CONSTRAINT "legacy_kit_challenge_kit_id_fkey" FOREIGN KEY (kit_id) REFERENCES legacy_kit(id) ON DELETE CASCADE,
    CONSTRAINT "legacy_kit_challenge_pkey" PRIMARY KEY (id)
);

CREATE INDEX idx_legacy_kit_challenge_kit_id ON legacy_kit_challenge (kit_id);

CREATE INDEX idx_legacy_kit_challenge_expires_at ON legacy_kit_challenge (expires_at);

CREATE TABLE "legacy_kit_recovery_session" (
    "id" TEXT NOT NULL,
    "kit_id" TEXT NOT NULL,
    "user_id" BIGINT NOT NULL CHECK ("user_id" IS NULL OR (typeof("user_id") = 'integer')),
    "status" TEXT NOT NULL,
    "effective_notice_period_in_hrs" BIGINT NOT NULL CHECK ("effective_notice_period_in_hrs" IS NULL OR (typeof("effective_notice_period_in_hrs") = 'integer' AND "effective_notice_period_in_hrs" BETWEEN -2147483648 AND 2147483647)),
    "wait_till" BIGINT NOT NULL CHECK ("wait_till" IS NULL OR (typeof("wait_till") = 'integer')),
    "next_reminder_at" BIGINT CHECK ("next_reminder_at" IS NULL OR (typeof("next_reminder_at") = 'integer')),
    "initiators" ANY NOT NULL DEFAULT ('[]') CHECK ("initiators" IS NULL OR (json_valid("initiators"))),
    "created_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("created_at" IS NULL OR (typeof("created_at") = 'integer')),
    "updated_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("updated_at" IS NULL OR (typeof("updated_at") = 'integer')),
    CONSTRAINT "legacy_kit_recovery_session_kit_id_fkey" FOREIGN KEY (kit_id) REFERENCES legacy_kit(id) ON DELETE CASCADE,
    CONSTRAINT "legacy_kit_recovery_session_pkey" PRIMARY KEY (id),
    CONSTRAINT "legacy_kit_recovery_session_status_check" CHECK ((status IN ('WAITING', 'READY', 'BLOCKED', 'CANCELLED', 'RECOVERED'))),
    CONSTRAINT "legacy_kit_recovery_session_user_id_fkey" FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX idx_legacy_kit_recovery_active_per_kit ON legacy_kit_recovery_session (kit_id) WHERE (status IN ('WAITING', 'READY'));

CREATE INDEX idx_legacy_kit_recovery_user_id ON legacy_kit_recovery_session (user_id);

CREATE INDEX idx_legacy_kit_recovery_kit_id ON legacy_kit_recovery_session (kit_id);

CREATE TABLE "legacy_kit_recovery_session_token" (
    "id" TEXT NOT NULL,
    "session_id" TEXT NOT NULL,
    "token_hash" TEXT NOT NULL,
    "created_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("created_at" IS NULL OR (typeof("created_at") = 'integer')),
    CONSTRAINT "legacy_kit_recovery_session_token_pkey" PRIMARY KEY (id),
    CONSTRAINT "legacy_kit_recovery_session_token_session_id_fkey" FOREIGN KEY (session_id) REFERENCES legacy_kit_recovery_session(id) ON DELETE CASCADE,
    CONSTRAINT "legacy_kit_recovery_session_token_token_hash_key" UNIQUE (token_hash)
);

CREATE INDEX idx_legacy_kit_recovery_session_token_session_id ON legacy_kit_recovery_session_token (session_id);

CREATE TABLE "memory_share_files" (
    "id" INTEGER PRIMARY KEY AUTOINCREMENT,
    "memory_share_id" BIGINT NOT NULL CHECK ("memory_share_id" IS NULL OR (typeof("memory_share_id") = 'integer')),
    "file_id" BIGINT NOT NULL CHECK ("file_id" IS NULL OR (typeof("file_id") = 'integer')),
    "file_owner_id" BIGINT NOT NULL CHECK ("file_owner_id" IS NULL OR (typeof("file_owner_id") = 'integer')),
    "position" BIGINT NOT NULL CHECK ("position" IS NULL OR (typeof("position") = 'integer')),
    "file_enc_key" TEXT NOT NULL,
    "file_key_decryption_nonce" TEXT NOT NULL,
    "created_at" BIGINT NOT NULL CHECK ("created_at" IS NULL OR (typeof("created_at") = 'integer')),
    CONSTRAINT "memory_share_files_memory_share_id_fkey" FOREIGN KEY (memory_share_id) REFERENCES memory_shares(id) ON DELETE CASCADE,
    CONSTRAINT "memory_share_files_memory_share_id_position_key" UNIQUE (memory_share_id, "position")
);

CREATE TABLE "memory_shares" (
    "id" INTEGER PRIMARY KEY AUTOINCREMENT,
    "user_id" BIGINT NOT NULL CHECK ("user_id" IS NULL OR (typeof("user_id") = 'integer')),
    "type" TEXT NOT NULL,
    "memory_hash" TEXT,
    "metadata_cipher" TEXT,
    "metadata_nonce" TEXT,
    "mem_enc_key" TEXT NOT NULL,
    "mem_key_decryption_nonce" TEXT NOT NULL,
    "access_token" TEXT NOT NULL,
    "is_deleted" BIGINT DEFAULT (false) CHECK ("is_deleted" IS NULL OR ("is_deleted" IN (0, 1))),
    "created_at" BIGINT NOT NULL CHECK ("created_at" IS NULL OR (typeof("created_at") = 'integer')),
    "updated_at" BIGINT NOT NULL CHECK ("updated_at" IS NULL OR (typeof("updated_at") = 'integer')),
    CONSTRAINT "memory_shares_access_token_key" UNIQUE (access_token),
    CONSTRAINT "memory_shares_type_check" CHECK ((type IN ('share', 'lane'))),
    CONSTRAINT "memory_shares_user_id_fkey" FOREIGN KEY (user_id) REFERENCES users(user_id)
);

CREATE TABLE "notification_history" (
    "user_id" BIGINT NOT NULL CHECK ("user_id" IS NULL OR (typeof("user_id") = 'integer')),
    "template_id" TEXT NOT NULL,
    "sent_time" BIGINT NOT NULL CHECK ("sent_time" IS NULL OR (typeof("sent_time") = 'integer')),
    "notification_group" TEXT,
    CONSTRAINT "fk_notification_history_user_id" FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE
);

CREATE INDEX notification_history_user_template_sent_time_idx ON notification_history (user_id, template_id, sent_time DESC);

CREATE INDEX notification_history_group_user_id_idx ON notification_history (notification_group, user_id);

CREATE TABLE "object_copies" (
    "object_key" TEXT NOT NULL,
    "b2" BIGINT CHECK ("b2" IS NULL OR (typeof("b2") = 'integer')),
    "want_b2" BIGINT CHECK ("want_b2" IS NULL OR ("want_b2" IN (0, 1))),
    "wasabi" BIGINT CHECK ("wasabi" IS NULL OR (typeof("wasabi") = 'integer')),
    "want_wasabi" BIGINT CHECK ("want_wasabi" IS NULL OR ("want_wasabi" IN (0, 1))),
    "scw" BIGINT CHECK ("scw" IS NULL OR (typeof("scw") = 'integer')),
    "want_scw" BIGINT CHECK ("want_scw" IS NULL OR ("want_scw" IN (0, 1))),
    "last_attempt" BIGINT NOT NULL DEFAULT (0) CHECK ("last_attempt" IS NULL OR (typeof("last_attempt") = 'integer')),
    CONSTRAINT "fk_object_copies_object_key" FOREIGN KEY (object_key) REFERENCES object_keys(object_key) ON DELETE CASCADE,
    CONSTRAINT "object_copies_pkey" PRIMARY KEY (object_key)
);

CREATE INDEX object_copies_wasabi_null_index ON object_copies (wasabi) WHERE ((wasabi IS NULL) AND (want_wasabi = true));

CREATE INDEX object_copies_scw_null_index ON object_copies (scw) WHERE ((scw IS NULL) AND (want_scw = true));

CREATE TABLE "object_keys" (
    "file_id" BIGINT NOT NULL CHECK ("file_id" IS NULL OR (typeof("file_id") = 'integer')),
    "o_type" TEXT NOT NULL CHECK ("o_type" IS NULL OR ("o_type" IN ('file', 'thumbnail', 'mldata', 'vid_preview', 'img_preview'))),
    "object_key" TEXT NOT NULL,
    "size" BIGINT NOT NULL CHECK ("size" IS NULL OR (typeof("size") = 'integer')),
    "datacenters" TEXT NOT NULL CHECK ("datacenters" IS NULL OR (json_valid("datacenters") AND json_type("datacenters") = 'array')),
    "is_deleted" BIGINT DEFAULT (false) CHECK ("is_deleted" IS NULL OR ("is_deleted" IN (0, 1))),
    "created_at" BIGINT DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("created_at" IS NULL OR (typeof("created_at") = 'integer')),
    "updated_at" BIGINT DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("updated_at" IS NULL OR (typeof("updated_at") = 'integer')),
    CONSTRAINT "fk_object_keys_file_id" FOREIGN KEY (file_id) REFERENCES files(file_id) ON DELETE CASCADE,
    CONSTRAINT "object_keys_object_key_key" UNIQUE (object_key),
    CONSTRAINT "object_keys_pkey" PRIMARY KEY (file_id, o_type)
);

CREATE TABLE "otts" (
    "ott" TEXT NOT NULL,
    "creation_time" BIGINT NOT NULL CHECK ("creation_time" IS NULL OR (typeof("creation_time") = 'integer')),
    "expiration_time" BIGINT NOT NULL CHECK ("expiration_time" IS NULL OR (typeof("expiration_time") = 'integer')),
    "email" TEXT,
    "email_hash" TEXT,
    "wrong_attempt" BIGINT NOT NULL DEFAULT (0) CHECK ("wrong_attempt" IS NULL OR (typeof("wrong_attempt") = 'integer' AND "wrong_attempt" BETWEEN -2147483648 AND 2147483647)),
    "app" TEXT NOT NULL DEFAULT ('photos') CHECK ("app" IS NULL OR ("app" IN ('photos', 'auth', 'locker'))),
    CONSTRAINT "unique_otts_emailhash_app_ott" UNIQUE (ott, app, email_hash)
);

CREATE INDEX otts_email_hash_index ON otts (email_hash);

CREATE TABLE "passkey_credentials" (
    "passkey_id" TEXT NOT NULL,
    "credential_id" TEXT NOT NULL,
    "public_key" TEXT NOT NULL,
    "attestation_type" TEXT NOT NULL,
    "authenticator_transports" TEXT NOT NULL,
    "credential_flags" TEXT NOT NULL,
    "authenticator" TEXT NOT NULL,
    "created_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("created_at" IS NULL OR (typeof("created_at") = 'integer')),
    "rp_id" TEXT,
    CONSTRAINT "fk_passkey_credentials_passkey_id" FOREIGN KEY (passkey_id) REFERENCES passkeys(id) ON DELETE CASCADE,
    CONSTRAINT "passkey_credentials_credential_id_key" UNIQUE (credential_id),
    CONSTRAINT "passkey_credentials_pkey" PRIMARY KEY (passkey_id)
);

CREATE TABLE "passkey_login_sessions" (
    "user_id" BIGINT NOT NULL CHECK ("user_id" IS NULL OR (typeof("user_id") = 'integer')),
    "session_id" TEXT NOT NULL,
    "creation_time" BIGINT NOT NULL CHECK ("creation_time" IS NULL OR (typeof("creation_time") = 'integer')),
    "expiration_time" BIGINT NOT NULL CHECK ("expiration_time" IS NULL OR (typeof("expiration_time") = 'integer')),
    "token_fetch_cnt" BIGINT DEFAULT (0) CHECK ("token_fetch_cnt" IS NULL OR (typeof("token_fetch_cnt") = 'integer' AND "token_fetch_cnt" BETWEEN -2147483648 AND 2147483647)),
    "verified_at" BIGINT CHECK ("verified_at" IS NULL OR (typeof("verified_at") = 'integer')),
    "token_data" BLOB CHECK ("token_data" IS NULL OR (typeof("token_data") = 'blob')),
    CONSTRAINT "fk_passkey_login_sessions_user_id" FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE,
    CONSTRAINT "passkey_login_sessions_session_id_key" UNIQUE (session_id)
);

CREATE TABLE "passkeys" (
    "id" TEXT NOT NULL,
    "user_id" BIGINT NOT NULL CHECK ("user_id" IS NULL OR (typeof("user_id") = 'integer')),
    "friendly_name" TEXT NOT NULL,
    "deleted_at" BIGINT CHECK ("deleted_at" IS NULL OR (typeof("deleted_at") = 'integer')),
    "created_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("created_at" IS NULL OR (typeof("created_at") = 'integer')),
    CONSTRAINT "fk_passkeys_user_id" FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE,
    CONSTRAINT "passkeys_pkey" PRIMARY KEY (id)
);

CREATE TABLE "public_abuse_report" (
    "share_id" BIGINT CHECK ("share_id" IS NULL OR (typeof("share_id") = 'integer')),
    "ip" TEXT NOT NULL,
    "user_agent" TEXT NOT NULL,
    "url" TEXT NOT NULL,
    "reason" TEXT NOT NULL,
    "created_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("created_at" IS NULL OR (typeof("created_at") = 'integer')),
    "details" ANY CHECK ("details" IS NULL OR (json_valid("details"))),
    CONSTRAINT "fk_public_abuse_report_token_id" FOREIGN KEY (share_id) REFERENCES public_collection_tokens(id) ON DELETE CASCADE,
    CONSTRAINT "unique_report_sid_ip_ua" UNIQUE (share_id, ip, user_agent)
);

CREATE INDEX public_abuse_share_id_idx ON public_abuse_report (share_id);

CREATE TABLE "public_collection_access_history" (
    "share_id" BIGINT CHECK ("share_id" IS NULL OR (typeof("share_id") = 'integer')),
    "ip" TEXT NOT NULL,
    "user_agent" TEXT NOT NULL,
    "created_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("created_at" IS NULL OR (typeof("created_at") = 'integer')),
    CONSTRAINT "fk_public_history_token_id" FOREIGN KEY (share_id) REFERENCES public_collection_tokens(id) ON DELETE CASCADE,
    CONSTRAINT "unique_access_sid_ip_ua" UNIQUE (share_id, ip, user_agent)
);

CREATE INDEX public_access_share_id_idx ON public_collection_access_history (share_id);

CREATE TABLE "public_collection_tokens" (
    "id" INTEGER PRIMARY KEY AUTOINCREMENT,
    "collection_id" BIGINT NOT NULL CHECK ("collection_id" IS NULL OR (typeof("collection_id") = 'integer')),
    "access_token" TEXT NOT NULL,
    "is_disabled" BIGINT NOT NULL DEFAULT (false) CHECK ("is_disabled" IS NULL OR ("is_disabled" IN (0, 1))),
    "valid_till" BIGINT NOT NULL DEFAULT (0) CHECK ("valid_till" IS NULL OR (typeof("valid_till") = 'integer')),
    "device_limit" BIGINT NOT NULL DEFAULT (0) CHECK ("device_limit" IS NULL OR (typeof("device_limit") = 'integer' AND "device_limit" BETWEEN -2147483648 AND 2147483647)),
    "created_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("created_at" IS NULL OR (typeof("created_at") = 'integer')),
    "updated_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("updated_at" IS NULL OR (typeof("updated_at") = 'integer')),
    "pw_hash" TEXT,
    "pw_nonce" TEXT,
    "mem_limit" BIGINT CHECK ("mem_limit" IS NULL OR (typeof("mem_limit") = 'integer')),
    "ops_limit" BIGINT CHECK ("ops_limit" IS NULL OR (typeof("ops_limit") = 'integer')),
    "enable_download" BIGINT NOT NULL DEFAULT (true) CHECK ("enable_download" IS NULL OR ("enable_download" IN (0, 1))),
    "enable_comment" BIGINT NOT NULL DEFAULT (false) CHECK ("enable_comment" IS NULL OR ("enable_comment" IN (0, 1))),
    "enable_collect" BIGINT NOT NULL DEFAULT (false) CHECK ("enable_collect" IS NULL OR ("enable_collect" IN (0, 1))),
    "enable_join" BIGINT NOT NULL DEFAULT (true) CHECK ("enable_join" IS NULL OR ("enable_join" IN (0, 1))),
    "min_role" TEXT CHECK ("min_role" IS NULL OR ("min_role" IN ('VIEWER', 'COLLABORATOR', 'OWNER', 'ADMIN'))),
    CONSTRAINT "fk_public_tokens_collection_id" FOREIGN KEY (collection_id) REFERENCES collections(collection_id) ON DELETE CASCADE,
    CONSTRAINT "pct_pw_state_constraint" CHECK ((((pw_hash IS NULL) AND (pw_nonce IS NULL)) OR ((pw_hash IS NOT NULL) AND (pw_nonce IS NOT NULL))))
);

CREATE UNIQUE INDEX public_active_collection_unique_idx ON public_collection_tokens (collection_id, is_disabled) WHERE (is_disabled = false);

CREATE UNIQUE INDEX public_access_tokens_unique_idx ON public_collection_tokens (access_token);

CREATE TABLE "public_file_tokens" (
    "id" TEXT NOT NULL,
    "file_id" BIGINT NOT NULL CHECK ("file_id" IS NULL OR (typeof("file_id") = 'integer')),
    "owner_id" BIGINT NOT NULL CHECK ("owner_id" IS NULL OR (typeof("owner_id") = 'integer')),
    "app" TEXT NOT NULL,
    "access_token" TEXT NOT NULL,
    "valid_till" BIGINT NOT NULL DEFAULT (0) CHECK ("valid_till" IS NULL OR (typeof("valid_till") = 'integer')),
    "device_limit" BIGINT NOT NULL DEFAULT (0) CHECK ("device_limit" IS NULL OR (typeof("device_limit") = 'integer' AND "device_limit" BETWEEN -2147483648 AND 2147483647)),
    "is_disabled" BIGINT NOT NULL DEFAULT (false) CHECK ("is_disabled" IS NULL OR ("is_disabled" IN (0, 1))),
    "enable_download" BIGINT NOT NULL DEFAULT (true) CHECK ("enable_download" IS NULL OR ("enable_download" IN (0, 1))),
    "pw_hash" TEXT,
    "pw_nonce" TEXT,
    "mem_limit" BIGINT CHECK ("mem_limit" IS NULL OR (typeof("mem_limit") = 'integer')),
    "ops_limit" BIGINT CHECK ("ops_limit" IS NULL OR (typeof("ops_limit") = 'integer')),
    "created_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("created_at" IS NULL OR (typeof("created_at") = 'integer')),
    "updated_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("updated_at" IS NULL OR (typeof("updated_at") = 'integer')),
    "encrypted_file_key" TEXT,
    "encrypted_file_key_nonce" TEXT,
    "kdf_nonce" TEXT,
    "kdf_mem_limit" BIGINT CHECK ("kdf_mem_limit" IS NULL OR (typeof("kdf_mem_limit") = 'integer')),
    "kdf_ops_limit" BIGINT CHECK ("kdf_ops_limit" IS NULL OR (typeof("kdf_ops_limit") = 'integer')),
    "encrypted_share_key" TEXT,
    CONSTRAINT "public_file_tokens_pkey" PRIMARY KEY (id)
);

CREATE UNIQUE INDEX public_file_token_unique_idx ON public_file_tokens (access_token) WHERE (is_disabled = false);

CREATE INDEX public_file_tokens_owner_id_updated_at_idx ON public_file_tokens (owner_id, updated_at);

CREATE UNIQUE INDEX public_active_file_link_unique_idx ON public_file_tokens (file_id, is_disabled) WHERE (is_disabled = false);

CREATE TABLE "public_file_tokens_access_history" (
    "id" TEXT NOT NULL,
    "ip" TEXT NOT NULL,
    "user_agent" TEXT NOT NULL,
    "created_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("created_at" IS NULL OR (typeof("created_at") = 'integer')),
    CONSTRAINT "fk_public_file_history_token_id" FOREIGN KEY (id) REFERENCES public_file_tokens(id) ON DELETE CASCADE,
    CONSTRAINT "unique_access_id_ip_ua" UNIQUE (id, ip, user_agent)
);

CREATE TABLE "public_paste_tokens" (
    "id" TEXT NOT NULL,
    "access_token" TEXT NOT NULL,
    "encrypted_data" TEXT NOT NULL,
    "decryption_header" TEXT NOT NULL,
    "encrypted_paste_key" TEXT NOT NULL,
    "encrypted_paste_key_nonce" TEXT NOT NULL,
    "kdf_nonce" TEXT NOT NULL,
    "kdf_mem_limit" BIGINT NOT NULL CHECK ("kdf_mem_limit" IS NULL OR (typeof("kdf_mem_limit") = 'integer')),
    "kdf_ops_limit" BIGINT NOT NULL CHECK ("kdf_ops_limit" IS NULL OR (typeof("kdf_ops_limit") = 'integer')),
    "expires_at" BIGINT NOT NULL CHECK ("expires_at" IS NULL OR (typeof("expires_at") = 'integer')),
    "created_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("created_at" IS NULL OR (typeof("created_at") = 'integer')),
    CONSTRAINT "public_paste_tokens_access_token_key" UNIQUE (access_token),
    CONSTRAINT "public_paste_tokens_pkey" PRIMARY KEY (id)
);

CREATE INDEX public_paste_tokens_expires_at_idx ON public_paste_tokens (expires_at);

CREATE TABLE "push_tokens" (
    "user_id" BIGINT NOT NULL CHECK ("user_id" IS NULL OR (typeof("user_id") = 'integer')),
    "fcm_token" TEXT NOT NULL,
    "apns_token" TEXT,
    "created_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("created_at" IS NULL OR (typeof("created_at") = 'integer')),
    "updated_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("updated_at" IS NULL OR (typeof("updated_at") = 'integer')),
    "last_notified_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("last_notified_at" IS NULL OR (typeof("last_notified_at") = 'integer')),
    "session_token_hash" BLOB CHECK ("session_token_hash" IS NULL OR (typeof("session_token_hash") = 'blob')),
    "platform" TEXT NOT NULL DEFAULT ('ios'),
    CONSTRAINT "fk_push_tokens_user_id" FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE,
    CONSTRAINT "push_tokens_pkey" PRIMARY KEY (fcm_token),
    CONSTRAINT "push_tokens_platform_check" CHECK ((platform IN ('ios', 'android')))
);

CREATE INDEX push_tokens_last_notified_at_index ON push_tokens (last_notified_at);

CREATE INDEX push_tokens_session_token_hash_idx ON push_tokens (session_token_hash) WHERE (session_token_hash IS NOT NULL);

CREATE TABLE "queue" (
    "queue_id" INTEGER PRIMARY KEY AUTOINCREMENT,
    "queue_name" TEXT NOT NULL,
    "item" TEXT NOT NULL,
    "created_at" BIGINT DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("created_at" IS NULL OR (typeof("created_at") = 'integer')),
    "updated_at" BIGINT DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("updated_at" IS NULL OR (typeof("updated_at") = 'integer')),
    "is_deleted" BIGINT DEFAULT (false) CHECK ("is_deleted" IS NULL OR ("is_deleted" IN (0, 1)))
);

CREATE UNIQUE INDEX name_and_item_unique_index ON queue (queue_name, item);

CREATE INDEX q_name_create_and_is_deleted_index ON queue (queue_name, created_at, is_deleted);

CREATE INDEX idx_queue_created_at_non_deleted ON queue (queue_name, created_at) WHERE (is_deleted = false);

CREATE TABLE "reactions" (
    "id" TEXT NOT NULL,
    "collection_id" BIGINT NOT NULL CHECK ("collection_id" IS NULL OR (typeof("collection_id") = 'integer')),
    "file_id" BIGINT CHECK ("file_id" IS NULL OR (typeof("file_id") = 'integer')),
    "comment_id" TEXT,
    "user_id" BIGINT NOT NULL CHECK ("user_id" IS NULL OR (typeof("user_id") = 'integer')),
    "anon_user_id" TEXT,
    "actor_key" TEXT GENERATED ALWAYS AS (
CASE
    WHEN ((user_id = '-1') AND (anon_user_id IS NOT NULL)) THEN ('A:' || anon_user_id)
    ELSE ('U:' || (user_id))
END) STORED,
    "cipher" TEXT,
    "nonce" TEXT,
    "is_deleted" BIGINT NOT NULL DEFAULT (false) CHECK ("is_deleted" IS NULL OR ("is_deleted" IN (0, 1))),
    "created_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("created_at" IS NULL OR (typeof("created_at") = 'integer')),
    "updated_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("updated_at" IS NULL OR (typeof("updated_at") = 'integer')),
    "unique_key" TEXT GENERATED ALWAYS AS (
CASE
    WHEN (comment_id IS NOT NULL) THEN ('M:' || comment_id)
    WHEN (file_id IS NOT NULL) THEN ((('F:' || (collection_id)) || '/') || (file_id))
    ELSE ('C:' || (collection_id))
END) STORED,
    CONSTRAINT "fk_reactions_anon_user" FOREIGN KEY (anon_user_id) REFERENCES anon_users(id),
    CONSTRAINT "reactions_cipher_on_delete" CHECK ((is_deleted = false AND cipher IS NOT NULL AND nonce IS NOT NULL) OR is_deleted = true),
    CONSTRAINT "reactions_pkey" PRIMARY KEY (id),
    CONSTRAINT "reactions_target_shape" CHECK ((((comment_id IS NOT NULL) AND (file_id IS NULL)) OR (comment_id IS NULL))),
    CONSTRAINT "reactions_user_or_anon_chk" CHECK ((((user_id = '-1') AND (anon_user_id IS NOT NULL)) OR ((user_id > 0) AND (anon_user_id IS NULL)))),
    CONSTRAINT "uq_reactions_actor_target" UNIQUE (actor_key, unique_key)
);

CREATE INDEX idx_reactions_scope ON reactions (collection_id, file_id, comment_id) WHERE (is_deleted = false);

CREATE INDEX idx_reactions_collection_updated_at ON reactions (collection_id, updated_at);

CREATE INDEX idx_reactions_comment ON reactions (comment_id) WHERE (is_deleted = false);

CREATE INDEX idx_reactions_user ON reactions (user_id) WHERE (is_deleted = false);

CREATE INDEX idx_reactions_anon ON reactions (anon_user_id) WHERE (is_deleted = false);

CREATE TABLE "referral_codes" (
    "code" TEXT NOT NULL CHECK ("code" IS NULL OR (length("code") <= 255)),
    "user_id" BIGINT NOT NULL CHECK ("user_id" IS NULL OR (typeof("user_id") = 'integer')),
    "is_active" BIGINT NOT NULL DEFAULT (true) CHECK ("is_active" IS NULL OR ("is_active" IN (0, 1))),
    "created_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("created_at" IS NULL OR (typeof("created_at") = 'integer')),
    CONSTRAINT "referral_codes_pkey" PRIMARY KEY (code)
);

CREATE UNIQUE INDEX referral_codes_user_id_is_active_idx ON referral_codes (user_id, is_active) WHERE (is_active = true);

CREATE TABLE "referral_tracking" (
    "invitor_id" BIGINT NOT NULL CHECK ("invitor_id" IS NULL OR (typeof("invitor_id") = 'integer')),
    "invitee_id" BIGINT NOT NULL CHECK ("invitee_id" IS NULL OR (typeof("invitee_id") = 'integer')),
    "plan_type" TEXT NOT NULL,
    "invitee_on_paid_plan" BIGINT DEFAULT (false) CHECK ("invitee_on_paid_plan" IS NULL OR ("invitee_on_paid_plan" IN (0, 1))),
    "created_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("created_at" IS NULL OR (typeof("created_at") = 'integer')),
    CONSTRAINT "referral_tracking_plan_type_check" CHECK ((plan_type = '10_GB_ON_UPGRADE'))
);

CREATE UNIQUE INDEX referral_tracking_invitee_id_idx ON referral_tracking (invitee_id);

CREATE TABLE "remote_store" (
    "user_id" BIGINT NOT NULL CHECK ("user_id" IS NULL OR (typeof("user_id") = 'integer')),
    "key_name" TEXT NOT NULL,
    "key_value" TEXT NOT NULL,
    "created_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("created_at" IS NULL OR (typeof("created_at") = 'integer')),
    "updated_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("updated_at" IS NULL OR (typeof("updated_at") = 'integer')),
    "canonical_value" TEXT,
    CONSTRAINT "fk_remote_store_user_id" FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE,
    CONSTRAINT "remote_store_pkey" PRIMARY KEY (user_id, key_name)
);

CREATE UNIQUE INDEX remote_store_custom_domain_unique_idx ON remote_store (key_value) WHERE (key_name = 'customDomain');

CREATE UNIQUE INDEX remote_store_custom_domain_canonical_unique_idx ON remote_store (canonical_value) WHERE ((key_name = 'customDomain') AND (canonical_value IS NOT NULL));

CREATE TABLE "space_browser_sessions" (
    "token_hash" BLOB NOT NULL CHECK ("token_hash" IS NULL OR (typeof("token_hash") = 'blob')),
    "user_id" BIGINT NOT NULL CHECK ("user_id" IS NULL OR (typeof("user_id") = 'integer')),
    "session_wrap_key" TEXT NOT NULL,
    "expires_at" BIGINT NOT NULL CHECK ("expires_at" IS NULL OR (typeof("expires_at") = 'integer')),
    "created_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("created_at" IS NULL OR (typeof("created_at") = 'integer')),
    "updated_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("updated_at" IS NULL OR (typeof("updated_at") = 'integer')),
    "last_used_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("last_used_at" IS NULL OR (typeof("last_used_at") = 'integer')),
    CONSTRAINT "space_browser_sessions_pkey" PRIMARY KEY (token_hash),
    CONSTRAINT "space_browser_sessions_user_id_fkey" FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE
);

CREATE INDEX idx_space_browser_sessions_user ON space_browser_sessions (user_id, last_used_at DESC);

CREATE INDEX idx_space_browser_sessions_expires ON space_browser_sessions (expires_at);

CREATE TABLE "space_friend_requests" (
    "request_id" INTEGER PRIMARY KEY AUTOINCREMENT,
    "requester_space_id" TEXT NOT NULL,
    "target_space_id" TEXT NOT NULL,
    "requester_friend_sealed_space_key" BLOB NOT NULL CHECK ("requester_friend_sealed_space_key" IS NULL OR (typeof("requester_friend_sealed_space_key") = 'blob')),
    "requester_key_version" BIGINT NOT NULL DEFAULT (1) CHECK ("requester_key_version" IS NULL OR (typeof("requester_key_version") = 'integer' AND "requester_key_version" BETWEEN -2147483648 AND 2147483647)),
    "created_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("created_at" IS NULL OR (typeof("created_at") = 'integer')),
    "updated_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("updated_at" IS NULL OR (typeof("updated_at") = 'integer')),
    CONSTRAINT "chk_space_friend_requests_distinct_spaces" CHECK ((requester_space_id <> target_space_id)),
    CONSTRAINT "space_friend_requests_requester_space_id_fkey" FOREIGN KEY (requester_space_id) REFERENCES spaces(space_id) ON DELETE CASCADE,
    CONSTRAINT "space_friend_requests_target_space_id_fkey" FOREIGN KEY (target_space_id) REFERENCES spaces(space_id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX uq_space_friend_requests_pair ON space_friend_requests (requester_space_id, target_space_id);

CREATE UNIQUE INDEX uq_space_friend_requests_unordered_pair ON space_friend_requests (MIN(requester_space_id, target_space_id), MAX(requester_space_id, target_space_id));

CREATE INDEX idx_space_friend_requests_target_created ON space_friend_requests (target_space_id, created_at DESC);

CREATE INDEX idx_space_friend_requests_requester_created ON space_friend_requests (requester_space_id, created_at DESC);

CREATE TABLE "space_friend_shares" (
    "space_id" TEXT NOT NULL,
    "friend_space_id" TEXT NOT NULL,
    "friend_sealed_space_key" BLOB NOT NULL CHECK ("friend_sealed_space_key" IS NULL OR (typeof("friend_sealed_space_key") = 'blob')),
    "key_version" BIGINT NOT NULL DEFAULT (1) CHECK ("key_version" IS NULL OR (typeof("key_version") = 'integer' AND "key_version" BETWEEN -2147483648 AND 2147483647)),
    "created_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("created_at" IS NULL OR (typeof("created_at") = 'integer')),
    "updated_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("updated_at" IS NULL OR (typeof("updated_at") = 'integer')),
    CONSTRAINT "chk_space_friend_shares_distinct" CHECK ((space_id <> friend_space_id)),
    CONSTRAINT "space_friend_shares_friend_space_id_fkey" FOREIGN KEY (friend_space_id) REFERENCES spaces(space_id) ON DELETE CASCADE,
    CONSTRAINT "space_friend_shares_pkey" PRIMARY KEY (space_id, friend_space_id),
    CONSTRAINT "space_friend_shares_space_id_fkey" FOREIGN KEY (space_id) REFERENCES spaces(space_id) ON DELETE CASCADE
);

CREATE INDEX idx_space_friend_shares_friend_space ON space_friend_shares (friend_space_id, created_at DESC);

CREATE TABLE "space_key_versions" (
    "space_id" TEXT NOT NULL,
    "version" BIGINT NOT NULL CHECK ("version" IS NULL OR (typeof("version") = 'integer' AND "version" BETWEEN -2147483648 AND 2147483647)),
    "root_wrapped_space_key" BLOB NOT NULL CHECK ("root_wrapped_space_key" IS NULL OR (typeof("root_wrapped_space_key") = 'blob')),
    "encrypted_profile" BLOB NOT NULL DEFAULT (X'') CHECK ("encrypted_profile" IS NULL OR (typeof("encrypted_profile") = 'blob')),
    "wrapped_prev_key" BLOB CHECK ("wrapped_prev_key" IS NULL OR (typeof("wrapped_prev_key") = 'blob')),
    "created_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("created_at" IS NULL OR (typeof("created_at") = 'integer')),
    CONSTRAINT "space_key_versions_pkey" PRIMARY KEY (space_id, version),
    CONSTRAINT "space_key_versions_space_id_fkey" FOREIGN KEY (space_id) REFERENCES spaces(space_id) ON DELETE CASCADE
);

CREATE INDEX idx_space_key_versions_space_created ON space_key_versions (space_id, created_at DESC);

CREATE TABLE "space_links" (
    "link_id" INTEGER PRIMARY KEY AUTOINCREMENT,
    "space_id" TEXT NOT NULL,
    "auth_key_hash" BLOB NOT NULL CHECK ("auth_key_hash" IS NULL OR (typeof("auth_key_hash") = 'blob')),
    "kdf_salt" BLOB NOT NULL CHECK ("kdf_salt" IS NULL OR (typeof("kdf_salt") = 'blob')),
    "kdf_mem_limit" BIGINT NOT NULL CHECK ("kdf_mem_limit" IS NULL OR (typeof("kdf_mem_limit") = 'integer')),
    "kdf_ops_limit" BIGINT NOT NULL CHECK ("kdf_ops_limit" IS NULL OR (typeof("kdf_ops_limit") = 'integer')),
    "key_version" BIGINT NOT NULL CHECK ("key_version" IS NULL OR (typeof("key_version") = 'integer' AND "key_version" BETWEEN -2147483648 AND 2147483647)),
    "encrypted_space_key" BLOB NOT NULL CHECK ("encrypted_space_key" IS NULL OR (typeof("encrypted_space_key") = 'blob')),
    "encrypted_access_key" BLOB NOT NULL CHECK ("encrypted_access_key" IS NULL OR (typeof("encrypted_access_key") = 'blob')),
    "active" BIGINT NOT NULL DEFAULT (true) CHECK ("active" IS NULL OR ("active" IN (0, 1))),
    "created_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("created_at" IS NULL OR (typeof("created_at") = 'integer')),
    "updated_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("updated_at" IS NULL OR (typeof("updated_at") = 'integer')),
    CONSTRAINT "chk_space_links_auth_key_hash" CHECK ((length(auth_key_hash) = 32)),
    CONSTRAINT "chk_space_links_encrypted_access_key" CHECK ((length(encrypted_access_key) > 0)),
    CONSTRAINT "chk_space_links_encrypted_space_key" CHECK ((length(encrypted_space_key) > 0)),
    CONSTRAINT "chk_space_links_kdf_mem_limit" CHECK ((kdf_mem_limit = 67108864)),
    CONSTRAINT "chk_space_links_kdf_ops_limit" CHECK ((kdf_ops_limit = 2)),
    CONSTRAINT "chk_space_links_kdf_salt" CHECK ((length(kdf_salt) = 16)),
    CONSTRAINT "chk_space_links_key_version" CHECK ((key_version > 0)),
    CONSTRAINT "space_links_auth_key_hash_key" UNIQUE (auth_key_hash),
    CONSTRAINT "space_links_space_id_fkey" FOREIGN KEY (space_id) REFERENCES spaces(space_id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX uq_space_links_active_space ON space_links (space_id) WHERE (active = true);

CREATE INDEX idx_space_links_space_created ON space_links (space_id, created_at DESC);

CREATE TABLE "space_messages" (
    "message_id" TEXT NOT NULL,
    "sender_space_id" TEXT NOT NULL,
    "recipient_space_id" TEXT NOT NULL,
    "thread_key" TEXT GENERATED ALWAYS AS (
CASE
    WHEN (sender_space_id < recipient_space_id) THEN ((sender_space_id || ':') || recipient_space_id)
    ELSE ((recipient_space_id || ':') || sender_space_id)
END) STORED,
    "kind" TEXT NOT NULL,
    "message_cipher" BLOB CHECK ("message_cipher" IS NULL OR (typeof("message_cipher") = 'blob')),
    "sender_encrypted_message_key" BLOB CHECK ("sender_encrypted_message_key" IS NULL OR (typeof("sender_encrypted_message_key") = 'blob')),
    "recipient_encrypted_message_key" BLOB CHECK ("recipient_encrypted_message_key" IS NULL OR (typeof("recipient_encrypted_message_key") = 'blob')),
    "reply_post_id" BIGINT CHECK ("reply_post_id" IS NULL OR (typeof("reply_post_id") = 'integer')),
    "reply_message_id" TEXT,
    "recipient_liked_at" BIGINT CHECK ("recipient_liked_at" IS NULL OR (typeof("recipient_liked_at") = 'integer')),
    "is_deleted" BIGINT NOT NULL DEFAULT (false) CHECK ("is_deleted" IS NULL OR ("is_deleted" IN (0, 1))),
    "created_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("created_at" IS NULL OR (typeof("created_at") = 'integer')),
    "updated_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("updated_at" IS NULL OR (typeof("updated_at") = 'integer')),
    "sender_encrypted_reaction" BLOB CHECK ("sender_encrypted_reaction" IS NULL OR (typeof("sender_encrypted_reaction") = 'blob')),
    "recipient_encrypted_reaction" BLOB CHECK ("recipient_encrypted_reaction" IS NULL OR (typeof("recipient_encrypted_reaction") = 'blob')),
    CONSTRAINT "chk_space_messages_cipher_shape" CHECK (is_deleted = true OR (kind IN ('regular','post_reply') AND message_cipher IS NOT NULL AND sender_encrypted_message_key IS NOT NULL AND recipient_encrypted_message_key IS NOT NULL)),
    CONSTRAINT "chk_space_messages_distinct_spaces" CHECK ((sender_space_id <> recipient_space_id)),
    CONSTRAINT "chk_space_messages_friend_added_shape" CHECK (((kind <> 'friend_added') OR ((reply_post_id IS NULL) AND (reply_message_id IS NULL) AND (recipient_liked_at IS NULL)))),
    CONSTRAINT "chk_space_messages_kind" CHECK ((kind IN ('regular', 'post_reply'))),
    CONSTRAINT "chk_space_messages_post_like_shape" CHECK (((kind <> 'post_like') OR ((reply_post_id IS NOT NULL) AND (reply_message_id IS NULL) AND (recipient_liked_at IS NULL)))),
    CONSTRAINT "chk_space_messages_post_reply_shape" CHECK (((kind <> 'post_reply') OR ((reply_post_id IS NOT NULL) AND (reply_message_id IS NULL)))),
    CONSTRAINT "chk_space_messages_reaction" CHECK (is_deleted = true OR (((sender_encrypted_reaction IS NULL) AND (recipient_encrypted_reaction IS NULL)) OR ((sender_encrypted_reaction IS NOT NULL) AND (recipient_encrypted_reaction IS NOT NULL) AND (length(sender_encrypted_reaction) = 304) AND (length(recipient_encrypted_reaction) = 304) AND (recipient_liked_at IS NOT NULL) AND (kind IN ('regular', 'post_reply')) AND (is_deleted = false)))),
    CONSTRAINT "chk_space_messages_recipient_like" CHECK (is_deleted = true OR ((recipient_liked_at IS NULL) OR (is_deleted = false))),
    CONSTRAINT "chk_space_messages_regular_shape" CHECK (((kind <> 'regular') OR (reply_post_id IS NULL))),
    CONSTRAINT "chk_space_messages_single_reply_target" CHECK (((reply_post_id IS NULL) OR (reply_message_id IS NULL))),
    CONSTRAINT "space_messages_pkey" PRIMARY KEY (message_id),
    CONSTRAINT "space_messages_recipient_space_id_fkey" FOREIGN KEY (recipient_space_id) REFERENCES spaces(space_id) ON DELETE CASCADE,
    CONSTRAINT "space_messages_reply_message_id_fkey" FOREIGN KEY (reply_message_id) REFERENCES space_messages(message_id) ON DELETE SET NULL,
    CONSTRAINT "space_messages_reply_post_id_fkey" FOREIGN KEY (reply_post_id) REFERENCES space_posts(post_id) ON DELETE SET NULL,
    CONSTRAINT "space_messages_sender_space_id_fkey" FOREIGN KEY (sender_space_id) REFERENCES spaces(space_id) ON DELETE CASCADE
);

CREATE INDEX idx_space_messages_sender_space_created ON space_messages (sender_space_id, created_at DESC, message_id DESC) WHERE (is_deleted = false);

CREATE INDEX idx_space_messages_recipient_space_created ON space_messages (recipient_space_id, created_at DESC, message_id DESC) WHERE (is_deleted = false);

CREATE INDEX idx_space_messages_thread_created ON space_messages (thread_key, created_at DESC, message_id DESC);

CREATE INDEX idx_space_messages_reply_post ON space_messages (reply_post_id) WHERE (reply_post_id IS NOT NULL);

CREATE UNIQUE INDEX uq_space_messages_post_like ON space_messages (reply_post_id, sender_space_id) WHERE (kind = 'post_like');

CREATE INDEX idx_space_messages_reply_message ON space_messages (reply_message_id) WHERE (reply_message_id IS NOT NULL);

CREATE INDEX idx_space_messages_sender_liked ON space_messages (sender_space_id, recipient_liked_at DESC, message_id DESC) WHERE ((recipient_liked_at IS NOT NULL) AND (is_deleted = false));

CREATE INDEX idx_space_messages_recipient_liked ON space_messages (recipient_space_id, recipient_liked_at DESC, message_id DESC) WHERE ((recipient_liked_at IS NOT NULL) AND (is_deleted = false));

CREATE TABLE "space_notification_read_markers" (
    "viewer_space_id" TEXT NOT NULL,
    "friend_space_id" TEXT NOT NULL,
    "read_at" BIGINT NOT NULL DEFAULT (0) CHECK ("read_at" IS NULL OR (typeof("read_at") = 'integer')),
    "created_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("created_at" IS NULL OR (typeof("created_at") = 'integer')),
    "updated_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("updated_at" IS NULL OR (typeof("updated_at") = 'integer')),
    CONSTRAINT "chk_space_notification_read_markers_distinct" CHECK ((viewer_space_id <> friend_space_id)),
    CONSTRAINT "space_notification_read_markers_friend_space_id_fkey" FOREIGN KEY (friend_space_id) REFERENCES spaces(space_id) ON DELETE CASCADE,
    CONSTRAINT "space_notification_read_markers_pkey" PRIMARY KEY (viewer_space_id, friend_space_id),
    CONSTRAINT "space_notification_read_markers_viewer_space_id_fkey" FOREIGN KEY (viewer_space_id) REFERENCES spaces(space_id) ON DELETE CASCADE
);

CREATE TABLE "space_notifications" (
    "notification_id" TEXT NOT NULL,
    "recipient_space_id" TEXT NOT NULL,
    "actor_space_id" TEXT NOT NULL,
    "kind" TEXT NOT NULL,
    "post_like_id" TEXT,
    "created_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("created_at" IS NULL OR (typeof("created_at") = 'integer')),
    "read_at" BIGINT CHECK ("read_at" IS NULL OR (typeof("read_at") = 'integer')),
    "friend_request_id" BIGINT CHECK ("friend_request_id" IS NULL OR (typeof("friend_request_id") = 'integer')),
    "poke_id" TEXT,
    CONSTRAINT "chk_space_notifications_target" CHECK ((((kind = 'post_like') AND (post_like_id IS NOT NULL) AND (friend_request_id IS NULL) AND (poke_id IS NULL)) OR ((kind = 'friend_request') AND (friend_request_id IS NOT NULL) AND (post_like_id IS NULL) AND (poke_id IS NULL)) OR ((kind = 'friend_accepted') AND (post_like_id IS NULL) AND (friend_request_id IS NULL) AND (poke_id IS NULL)) OR ((kind = 'poke') AND (poke_id IS NOT NULL) AND (post_like_id IS NULL) AND (friend_request_id IS NULL)))),
    CONSTRAINT "space_notifications_actor_space_id_fkey" FOREIGN KEY (actor_space_id) REFERENCES spaces(space_id) ON DELETE CASCADE,
    CONSTRAINT "space_notifications_check" CHECK ((recipient_space_id <> actor_space_id)),
    CONSTRAINT "space_notifications_friend_request_id_fkey" FOREIGN KEY (friend_request_id) REFERENCES space_friend_requests(request_id) ON DELETE CASCADE,
    CONSTRAINT "space_notifications_pkey" PRIMARY KEY (notification_id),
    CONSTRAINT "space_notifications_poke_id_fkey" FOREIGN KEY (poke_id) REFERENCES space_pokes(poke_id) ON DELETE CASCADE,
    CONSTRAINT "space_notifications_post_like_id_fkey" FOREIGN KEY (post_like_id) REFERENCES space_post_likes(like_id) ON DELETE CASCADE,
    CONSTRAINT "space_notifications_recipient_space_id_fkey" FOREIGN KEY (recipient_space_id) REFERENCES spaces(space_id) ON DELETE CASCADE,
    CONSTRAINT "space_notifications_recipient_space_id_kind_post_like_id_key" UNIQUE (recipient_space_id, kind, post_like_id)
);

CREATE INDEX idx_space_notifications_post_like ON space_notifications (post_like_id);

CREATE INDEX idx_space_notifications_recipient_created ON space_notifications (recipient_space_id, created_at DESC, notification_id DESC);

CREATE INDEX idx_space_notifications_unread ON space_notifications (recipient_space_id) WHERE (read_at IS NULL);

CREATE UNIQUE INDEX idx_space_notifications_friend_request ON space_notifications (friend_request_id) WHERE (friend_request_id IS NOT NULL);

CREATE UNIQUE INDEX idx_space_notifications_poke ON space_notifications (poke_id);

CREATE TABLE "space_pokes" (
    "poke_id" TEXT NOT NULL,
    "sender_space_id" TEXT NOT NULL,
    "recipient_space_id" TEXT NOT NULL,
    "client_request_id" TEXT NOT NULL,
    "created_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("created_at" IS NULL OR (typeof("created_at") = 'integer')),
    CONSTRAINT "space_pokes_check" CHECK ((sender_space_id <> recipient_space_id)),
    CONSTRAINT "space_pokes_pkey" PRIMARY KEY (poke_id),
    CONSTRAINT "space_pokes_recipient_space_id_fkey" FOREIGN KEY (recipient_space_id) REFERENCES spaces(space_id) ON DELETE CASCADE,
    CONSTRAINT "space_pokes_sender_space_id_fkey" FOREIGN KEY (sender_space_id) REFERENCES spaces(space_id) ON DELETE CASCADE,
    CONSTRAINT "space_pokes_sender_space_id_recipient_space_id_client_reque_key" UNIQUE (sender_space_id, recipient_space_id, client_request_id)
);

CREATE INDEX idx_space_pokes_recipient_created ON space_pokes (recipient_space_id, created_at DESC);

CREATE TABLE "space_post_assets" (
    "asset_id" INTEGER PRIMARY KEY AUTOINCREMENT,
    "post_id" BIGINT NOT NULL CHECK ("post_id" IS NULL OR (typeof("post_id") = 'integer')),
    "object_key" TEXT NOT NULL,
    "bucket_id" TEXT NOT NULL,
    "size" BIGINT CHECK ("size" IS NULL OR (typeof("size") = 'integer')),
    "position" BIGINT NOT NULL DEFAULT (0) CHECK ("position" IS NULL OR (typeof("position") = 'integer' AND "position" BETWEEN -2147483648 AND 2147483647)),
    "metadata_cipher" BLOB NOT NULL CHECK ("metadata_cipher" IS NULL OR (typeof("metadata_cipher") = 'blob')),
    "created_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("created_at" IS NULL OR (typeof("created_at") = 'integer')),
    "role" TEXT NOT NULL DEFAULT ('preview'),
    CONSTRAINT "chk_space_post_assets_bucket_id" CHECK ((bucket_id <> '')),
    CONSTRAINT "chk_space_post_assets_object_key" CHECK ((object_key <> '')),
    CONSTRAINT "chk_space_post_assets_position" CHECK (("position" >= 0)),
    CONSTRAINT "chk_space_post_assets_role" CHECK ((role IN ('preview', 'video'))),
    CONSTRAINT "space_post_assets_post_id_fkey" FOREIGN KEY (post_id) REFERENCES space_posts(post_id) ON DELETE CASCADE,
    CONSTRAINT "uq_space_post_assets_object_key" UNIQUE (object_key)
);

CREATE INDEX idx_space_post_assets_post_position ON space_post_assets (post_id, "position", asset_id);

CREATE UNIQUE INDEX uq_space_post_assets_position ON space_post_assets (post_id, "position", role);

CREATE TABLE "space_post_likes" (
    "like_id" TEXT NOT NULL,
    "post_id" BIGINT NOT NULL CHECK ("post_id" IS NULL OR (typeof("post_id") = 'integer')),
    "actor_space_id" TEXT NOT NULL,
    "created_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("created_at" IS NULL OR (typeof("created_at") = 'integer')),
    CONSTRAINT "space_post_likes_actor_space_id_fkey" FOREIGN KEY (actor_space_id) REFERENCES spaces(space_id) ON DELETE CASCADE,
    CONSTRAINT "space_post_likes_pkey" PRIMARY KEY (like_id),
    CONSTRAINT "space_post_likes_post_id_actor_space_id_key" UNIQUE (post_id, actor_space_id),
    CONSTRAINT "space_post_likes_post_id_fkey" FOREIGN KEY (post_id) REFERENCES space_posts(post_id) ON DELETE CASCADE
);

CREATE TABLE "space_posts" (
    "post_id" INTEGER PRIMARY KEY AUTOINCREMENT,
    "space_id" TEXT NOT NULL,
    "encrypted_post_key" BLOB NOT NULL CHECK ("encrypted_post_key" IS NULL OR (typeof("encrypted_post_key") = 'blob')),
    "caption_cipher" BLOB NOT NULL DEFAULT (X'') CHECK ("caption_cipher" IS NULL OR (typeof("caption_cipher") = 'blob')),
    "key_version" BIGINT NOT NULL DEFAULT (1) CHECK ("key_version" IS NULL OR (typeof("key_version") = 'integer' AND "key_version" BETWEEN -2147483648 AND 2147483647)),
    "is_deleted" BIGINT NOT NULL DEFAULT (false) CHECK ("is_deleted" IS NULL OR ("is_deleted" IN (0, 1))),
    "created_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("created_at" IS NULL OR (typeof("created_at") = 'integer')),
    "updated_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("updated_at" IS NULL OR (typeof("updated_at") = 'integer')),
    "client_request_id" TEXT,
    CONSTRAINT "space_posts_space_id_fkey" FOREIGN KEY (space_id) REFERENCES spaces(space_id) ON DELETE CASCADE
);

CREATE INDEX idx_space_posts_space_created ON space_posts (space_id, created_at DESC) WHERE (is_deleted = false);

CREATE UNIQUE INDEX uq_space_posts_client_request ON space_posts (space_id, client_request_id) WHERE (client_request_id IS NOT NULL);

CREATE TABLE "space_profile_assets" (
    "space_id" TEXT NOT NULL,
    "asset_type" TEXT NOT NULL,
    "object_id" TEXT NOT NULL,
    "bucket_id" TEXT NOT NULL,
    "size" BIGINT CHECK ("size" IS NULL OR (typeof("size") = 'integer')),
    "key_version" BIGINT NOT NULL CHECK ("key_version" IS NULL OR (typeof("key_version") = 'integer' AND "key_version" BETWEEN -2147483648 AND 2147483647)),
    "created_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("created_at" IS NULL OR (typeof("created_at") = 'integer')),
    "updated_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("updated_at" IS NULL OR (typeof("updated_at") = 'integer')),
    CONSTRAINT "chk_space_profile_assets_object_id" CHECK (((object_id <> '') AND (object_id NOT LIKE '%/%'))),
    CONSTRAINT "chk_space_profile_assets_size" CHECK (((size IS NULL) OR (size > 0))),
    CONSTRAINT "chk_space_profile_assets_type" CHECK ((asset_type IN ('avatar', 'cover'))),
    CONSTRAINT "space_profile_assets_pkey" PRIMARY KEY (space_id, asset_type),
    CONSTRAINT "space_profile_assets_space_id_fkey" FOREIGN KEY (space_id) REFERENCES spaces(space_id) ON DELETE CASCADE
);

CREATE TABLE "space_temp_objects" (
    "object_key" TEXT NOT NULL,
    "space_id" TEXT,
    "purpose" TEXT NOT NULL,
    "bucket_id" TEXT NOT NULL,
    "expected_size" BIGINT NOT NULL CHECK ("expected_size" IS NULL OR (typeof("expected_size") = 'integer')),
    "expires_at" BIGINT NOT NULL CHECK ("expires_at" IS NULL OR (typeof("expires_at") = 'integer')),
    "cleanup_after" BIGINT NOT NULL CHECK ("cleanup_after" IS NULL OR (typeof("cleanup_after") = 'integer')),
    "created_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("created_at" IS NULL OR (typeof("created_at") = 'integer')),
    "content_md5" TEXT,
    "client" TEXT CHECK ("client" IS NULL OR (length("client") <= 256)),
    CONSTRAINT "chk_space_temp_objects_expected_size" CHECK ((expected_size > 0)),
    CONSTRAINT "chk_space_temp_objects_purpose" CHECK ((purpose IN ('post', 'avatar', 'cover'))),
    CONSTRAINT "space_temp_objects_pkey" PRIMARY KEY (object_key),
    CONSTRAINT "space_temp_objects_space_id_fkey" FOREIGN KEY (space_id) REFERENCES spaces(space_id) ON DELETE CASCADE
);

CREATE INDEX idx_space_temp_objects_expires ON space_temp_objects (cleanup_after);

CREATE INDEX idx_space_temp_objects_active ON space_temp_objects (space_id, expires_at) WHERE (space_id IS NOT NULL);

CREATE TABLE "space_web_push_subscriptions" (
    "target_id" TEXT NOT NULL,
    "endpoint" TEXT NOT NULL,
    "session_token_hash" BLOB CHECK ("session_token_hash" IS NULL OR (typeof("session_token_hash") = 'blob')),
    "link_id" BIGINT CHECK ("link_id" IS NULL OR (typeof("link_id") = 'integer')),
    "p256dh" TEXT NOT NULL,
    "auth" TEXT NOT NULL,
    "created_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("created_at" IS NULL OR (typeof("created_at") = 'integer')),
    "updated_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("updated_at" IS NULL OR (typeof("updated_at") = 'integer')),
    CONSTRAINT "chk_space_web_push_subscription_target" CHECK ((((session_token_hash IS NOT NULL) AND (link_id IS NULL)) OR ((session_token_hash IS NULL) AND (link_id IS NOT NULL)))),
    CONSTRAINT "space_web_push_subscriptions_link_id_fkey" FOREIGN KEY (link_id) REFERENCES space_links(link_id) ON DELETE CASCADE,
    CONSTRAINT "space_web_push_subscriptions_pkey" PRIMARY KEY (target_id),
    CONSTRAINT "space_web_push_subscriptions_session_token_hash_fkey" FOREIGN KEY (session_token_hash) REFERENCES space_browser_sessions(token_hash) ON DELETE CASCADE,
    CONSTRAINT "space_web_push_subscriptions_session_token_hash_key" UNIQUE (session_token_hash),
    CONSTRAINT "uq_space_web_push_link" UNIQUE (endpoint, link_id)
);

CREATE UNIQUE INDEX uq_space_web_push_account_endpoint ON space_web_push_subscriptions (endpoint) WHERE (session_token_hash IS NOT NULL);

CREATE INDEX idx_space_web_push_link ON space_web_push_subscriptions (link_id) WHERE (link_id IS NOT NULL);

CREATE TABLE "spaces" (
    "space_id" TEXT NOT NULL,
    "owner_id" BIGINT NOT NULL CHECK ("owner_id" IS NULL OR (typeof("owner_id") = 'integer')),
    "space_slug" TEXT NOT NULL,
    "root_wrapped_space_key" BLOB NOT NULL CHECK ("root_wrapped_space_key" IS NULL OR (typeof("root_wrapped_space_key") = 'blob')),
    "public_key" BLOB NOT NULL CHECK ("public_key" IS NULL OR (typeof("public_key") = 'blob')),
    "encrypted_secret_key" BLOB NOT NULL CHECK ("encrypted_secret_key" IS NULL OR (typeof("encrypted_secret_key") = 'blob')),
    "encrypted_profile" BLOB NOT NULL DEFAULT (X'') CHECK ("encrypted_profile" IS NULL OR (typeof("encrypted_profile") = 'blob')),
    "current_version" BIGINT NOT NULL DEFAULT (1) CHECK ("current_version" IS NULL OR (typeof("current_version") = 'integer' AND "current_version" BETWEEN -2147483648 AND 2147483647)),
    "referred_by_space_id" TEXT,
    "created_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("created_at" IS NULL OR (typeof("created_at") = 'integer')),
    "updated_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("updated_at" IS NULL OR (typeof("updated_at") = 'integer')),
    CONSTRAINT "chk_spaces_referral_self" CHECK (((referred_by_space_id IS NULL) OR (referred_by_space_id <> space_id))),
    CONSTRAINT "spaces_owner_id_fkey" FOREIGN KEY (owner_id) REFERENCES users(user_id) ON DELETE CASCADE,
    CONSTRAINT "spaces_pkey" PRIMARY KEY (space_id),
    CONSTRAINT "spaces_referred_by_space_id_fkey" FOREIGN KEY (referred_by_space_id) REFERENCES spaces(space_id) ON DELETE SET NULL,
    CONSTRAINT "uq_spaces_slug" UNIQUE (space_slug)
);

CREATE INDEX idx_spaces_owner ON spaces (owner_id);

CREATE TABLE "srp_auth" (
    "user_id" BIGINT NOT NULL CHECK ("user_id" IS NULL OR (typeof("user_id") = 'integer')),
    "srp_user_id" TEXT NOT NULL,
    "salt" TEXT NOT NULL,
    "verifier" TEXT NOT NULL,
    "created_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("created_at" IS NULL OR (typeof("created_at") = 'integer')),
    "updated_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("updated_at" IS NULL OR (typeof("updated_at") = 'integer')),
    CONSTRAINT "fk_srp_auth_user_id" FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE,
    CONSTRAINT "srp_auth_pkey" PRIMARY KEY (user_id),
    CONSTRAINT "srp_auth_srp_user_id_key" UNIQUE (srp_user_id)
);

CREATE TABLE "srp_sessions" (
    "id" TEXT NOT NULL,
    "srp_user_id" TEXT NOT NULL,
    "server_key" TEXT NOT NULL,
    "srp_a" TEXT NOT NULL,
    "has_verified" BIGINT NOT NULL DEFAULT (false) CHECK ("has_verified" IS NULL OR ("has_verified" IN (0, 1))),
    "attempt_count" BIGINT NOT NULL DEFAULT (0) CHECK ("attempt_count" IS NULL OR (typeof("attempt_count") = 'integer' AND "attempt_count" BETWEEN -2147483648 AND 2147483647)),
    "created_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("created_at" IS NULL OR (typeof("created_at") = 'integer')),
    "updated_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("updated_at" IS NULL OR (typeof("updated_at") = 'integer')),
    "is_fake" BIGINT NOT NULL DEFAULT (false) CHECK ("is_fake" IS NULL OR ("is_fake" IN (0, 1))),
    CONSTRAINT "srp_sessions_pkey" PRIMARY KEY (id)
);

CREATE INDEX idx_srp_sessions_fake_created ON srp_sessions (is_fake, created_at) WHERE (is_fake = true);

CREATE TABLE "storage_bonus" (
    "bonus_id" TEXT NOT NULL,
    "type" TEXT NOT NULL,
    "user_id" BIGINT NOT NULL CHECK ("user_id" IS NULL OR (typeof("user_id") = 'integer')),
    "storage" BIGINT NOT NULL CHECK ("storage" IS NULL OR (typeof("storage") = 'integer')),
    "valid_till" BIGINT NOT NULL DEFAULT (0) CHECK ("valid_till" IS NULL OR (typeof("valid_till") = 'integer')),
    "revoke_reason" TEXT,
    "is_revoked" BIGINT NOT NULL DEFAULT (false) CHECK ("is_revoked" IS NULL OR ("is_revoked" IN (0, 1))),
    "created_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("created_at" IS NULL OR (typeof("created_at") = 'integer')),
    "updated_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("updated_at" IS NULL OR (typeof("updated_at") = 'integer')),
    CONSTRAINT "storage_bonus_pkey" PRIMARY KEY (bonus_id),
    CONSTRAINT "storage_bonus_type_check" CHECK ((type IN ('REFERRAL', 'SIGN_UP', 'ANNIVERSARY', 'ADD_ON_BF_2023', 'ADD_ON_SUPPORT', 'ADD_ON_BF_2024', 'ADD_ON_NON_PROFIT')))
);

CREATE INDEX storage_bonus_user_id_idx ON storage_bonus (user_id);

CREATE TABLE "subscription_logs" (
    "log_id" INTEGER PRIMARY KEY AUTOINCREMENT,
    "user_id" BIGINT NOT NULL CHECK ("user_id" IS NULL OR (typeof("user_id") = 'integer')),
    "payment_provider" TEXT NOT NULL,
    "notification" ANY NOT NULL CHECK ("notification" IS NULL OR (json_valid("notification"))),
    "verification_response" ANY NOT NULL CHECK ("verification_response" IS NULL OR (json_valid("verification_response"))),
    "created_at" TEXT NOT NULL DEFAULT (CURRENT_TIMESTAMP),
    CONSTRAINT "fk_subscription_logs_user_id" FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE
);

CREATE TABLE "subscriptions" (
    "subscription_id" INTEGER PRIMARY KEY AUTOINCREMENT,
    "user_id" BIGINT NOT NULL CHECK ("user_id" IS NULL OR (typeof("user_id") = 'integer')),
    "storage" BIGINT NOT NULL CHECK ("storage" IS NULL OR (typeof("storage") = 'integer')),
    "original_transaction_id" TEXT NOT NULL,
    "expiry_time" BIGINT NOT NULL CHECK ("expiry_time" IS NULL OR (typeof("expiry_time") = 'integer')),
    "created_at" TEXT NOT NULL DEFAULT (CURRENT_TIMESTAMP),
    "product_id" TEXT NOT NULL,
    "payment_provider" TEXT NOT NULL,
    "latest_verification_data" TEXT,
    "attributes" ANY NOT NULL CHECK ("attributes" IS NULL OR (json_valid("attributes"))),
    "upgraded_at" BIGINT DEFAULT (0) CHECK ("upgraded_at" IS NULL OR (typeof("upgraded_at") = 'integer')),
    CONSTRAINT "fk_subscriptions_user_id" FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE,
    CONSTRAINT "subscription_user_id_unique_constraint_index" UNIQUE (user_id)
);

CREATE INDEX subscriptions_expiry_time_index ON subscriptions (expiry_time);

CREATE INDEX subscriptions_user_id_index ON subscriptions (user_id);

CREATE UNIQUE INDEX sub_original_txn_id_index ON subscriptions (original_transaction_id) WHERE ((original_transaction_id IS NOT NULL) AND (original_transaction_id <> 'none'));

CREATE TABLE "task_lock" (
    "task_name" TEXT NOT NULL,
    "lock_until" BIGINT NOT NULL CHECK ("lock_until" IS NULL OR (typeof("lock_until") = 'integer')),
    "locked_at" BIGINT NOT NULL CHECK ("locked_at" IS NULL OR (typeof("locked_at") = 'integer')),
    "locked_by" TEXT NOT NULL,
    CONSTRAINT "task_lock_pkey" PRIMARY KEY (task_name)
);

CREATE INDEX task_lock_locked_until ON task_lock (lock_until);

CREATE TABLE "temp_objects" (
    "object_key" TEXT NOT NULL,
    "expiration_time" BIGINT NOT NULL CHECK ("expiration_time" IS NULL OR (typeof("expiration_time") = 'integer')),
    "is_multipart" BIGINT NOT NULL DEFAULT (false) CHECK ("is_multipart" IS NULL OR ("is_multipart" IN (0, 1))),
    "upload_id" TEXT,
    "bucket_id" TEXT CHECK ("bucket_id" IS NULL OR ("bucket_id" IN ('b2-eu-cen', 'scw-eu-fr', 'scw-eu-fr-locked', 'wasabi-eu-central-2', 'wasabi-eu-central-2-v3', 'scw-eu-fr-v3', 'wasabi-eu-central-2-derived', 'b5', 'b6'))),
    "user_id" BIGINT CHECK ("user_id" IS NULL OR (typeof("user_id") = 'integer')),
    "created_at" BIGINT DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("created_at" IS NULL OR (typeof("created_at") = 'integer')),
    "app" TEXT,
    "purpose" TEXT,
    "content_length" BIGINT CHECK ("content_length" IS NULL OR (typeof("content_length") = 'integer')),
    "content_md5" TEXT,
    "client" TEXT CHECK ("client" IS NULL OR (length("client") <= 256)),
    CONSTRAINT "temp_object_keys_pkey" PRIMARY KEY (object_key)
);

CREATE INDEX temp_objects_expiration_time_idx ON temp_objects (expiration_time);

CREATE INDEX temp_objects_user_created_idx ON temp_objects (user_id, created_at);

CREATE TABLE "temp_srp_setup" (
    "id" TEXT NOT NULL,
    "session_id" TEXT NOT NULL,
    "srp_user_id" TEXT NOT NULL,
    "user_id" BIGINT NOT NULL CHECK ("user_id" IS NULL OR (typeof("user_id") = 'integer')),
    "salt" TEXT NOT NULL,
    "verifier" TEXT NOT NULL,
    "created_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("created_at" IS NULL OR (typeof("created_at") = 'integer')),
    CONSTRAINT "fk_temp_srp_setup_user_id" FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE,
    CONSTRAINT "temp_srp_setup_pkey" PRIMARY KEY (id)
);

CREATE TABLE "temp_two_factor" (
    "user_id" BIGINT NOT NULL CHECK ("user_id" IS NULL OR (typeof("user_id") = 'integer')),
    "two_factor_secret_hash" TEXT,
    "encrypted_two_factor_secret" BLOB CHECK ("encrypted_two_factor_secret" IS NULL OR (typeof("encrypted_two_factor_secret") = 'blob')),
    "two_factor_secret_decryption_nonce" BLOB CHECK ("two_factor_secret_decryption_nonce" IS NULL OR (typeof("two_factor_secret_decryption_nonce") = 'blob')),
    "creation_time" BIGINT NOT NULL CHECK ("creation_time" IS NULL OR (typeof("creation_time") = 'integer')),
    "expiration_time" BIGINT NOT NULL CHECK ("expiration_time" IS NULL OR (typeof("expiration_time") = 'integer')),
    CONSTRAINT "fk_two_factor_user_id" FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE,
    CONSTRAINT "temp_two_factor_two_factor_secret_hash_key" UNIQUE (two_factor_secret_hash)
);

CREATE TABLE "tokens" (
    "user_id" BIGINT NOT NULL CHECK ("user_id" IS NULL OR (typeof("user_id") = 'integer')),
    "token" TEXT,
    "creation_time" BIGINT NOT NULL CHECK ("creation_time" IS NULL OR (typeof("creation_time") = 'integer')),
    "ip" TEXT,
    "user_agent" TEXT,
    "is_deleted" BIGINT NOT NULL DEFAULT (false) CHECK ("is_deleted" IS NULL OR ("is_deleted" IN (0, 1))),
    "last_used_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("last_used_at" IS NULL OR (typeof("last_used_at") = 'integer')),
    "app" TEXT NOT NULL DEFAULT ('photos') CHECK ("app" IS NULL OR ("app" IN ('photos', 'auth', 'locker'))),
    "token_hash" BLOB NOT NULL CHECK ("token_hash" IS NULL OR (typeof("token_hash") = 'blob')),
    CONSTRAINT "fk_tokens_user_id" FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE,
    CONSTRAINT "tokens_token_hash_length" CHECK ((length(token_hash) = 32)),
    CONSTRAINT "tokens_token_key" UNIQUE (token)
);

CREATE INDEX tokens_user_id_index ON tokens (user_id);

CREATE UNIQUE INDEX tokens_token_hash_unique_idx ON tokens (token_hash);

CREATE TABLE "trash" (
    "file_id" BIGINT NOT NULL CHECK ("file_id" IS NULL OR (typeof("file_id") = 'integer')),
    "user_id" BIGINT NOT NULL CHECK ("user_id" IS NULL OR (typeof("user_id") = 'integer')),
    "collection_id" BIGINT NOT NULL CHECK ("collection_id" IS NULL OR (typeof("collection_id") = 'integer')),
    "is_deleted" BIGINT NOT NULL DEFAULT (false) CHECK ("is_deleted" IS NULL OR ("is_deleted" IN (0, 1))),
    "is_restored" BIGINT NOT NULL DEFAULT (false) CHECK ("is_restored" IS NULL OR ("is_restored" IN (0, 1))),
    "created_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("created_at" IS NULL OR (typeof("created_at") = 'integer')),
    "updated_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("updated_at" IS NULL OR (typeof("updated_at") = 'integer')),
    "delete_by" BIGINT NOT NULL CHECK ("delete_by" IS NULL OR (typeof("delete_by") = 'integer')),
    CONSTRAINT "fk_trash_keys_collection_files" FOREIGN KEY (file_id, collection_id) REFERENCES collection_files(file_id, collection_id),
    CONSTRAINT "trash_pkey" PRIMARY KEY (file_id),
    CONSTRAINT "trash_state_constraint" CHECK (((is_deleted IS FALSE) OR (is_restored IS FALSE)))
);

CREATE INDEX trash_updated_at_time_index ON trash (updated_at);

CREATE INDEX trash_delete_by_idx ON trash (delete_by) WHERE ((is_deleted IS FALSE) AND (is_restored IS FALSE));

CREATE INDEX trash_user_id_and_updated_at_index ON trash (user_id, updated_at);

CREATE TABLE "two_factor" (
    "user_id" BIGINT NOT NULL CHECK ("user_id" IS NULL OR (typeof("user_id") = 'integer')),
    "two_factor_secret_hash" TEXT,
    "encrypted_two_factor_secret" BLOB CHECK ("encrypted_two_factor_secret" IS NULL OR (typeof("encrypted_two_factor_secret") = 'blob')),
    "two_factor_secret_decryption_nonce" BLOB CHECK ("two_factor_secret_decryption_nonce" IS NULL OR (typeof("two_factor_secret_decryption_nonce") = 'blob')),
    "recovery_encrypted_two_factor_secret" TEXT,
    "recovery_two_factor_secret_decryption_nonce" TEXT,
    CONSTRAINT "fk_two_factor_user_id" FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE,
    CONSTRAINT "two_factor_two_factor_secret_hash_key" UNIQUE (two_factor_secret_hash),
    CONSTRAINT "two_factor_user_id_key" UNIQUE (user_id)
);

CREATE TABLE "two_factor_recovery" (
    "user_id" BIGINT NOT NULL CHECK ("user_id" IS NULL OR (typeof("user_id") = 'integer')),
    "enable_admin_mfa_reset" BIGINT NOT NULL DEFAULT (true) CHECK ("enable_admin_mfa_reset" IS NULL OR ("enable_admin_mfa_reset" IN (0, 1))),
    "server_passkey_secret_data" BLOB CHECK ("server_passkey_secret_data" IS NULL OR (typeof("server_passkey_secret_data") = 'blob')),
    "server_passkey_secret_nonce" BLOB CHECK ("server_passkey_secret_nonce" IS NULL OR (typeof("server_passkey_secret_nonce") = 'blob')),
    "user_passkey_secret_data" TEXT,
    "user_passkey_secret_nonce" TEXT,
    "created_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("created_at" IS NULL OR (typeof("created_at") = 'integer')),
    "updated_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("updated_at" IS NULL OR (typeof("updated_at") = 'integer')),
    CONSTRAINT "two_factor_recovery_pkey" PRIMARY KEY (user_id)
);

CREATE TABLE "two_factor_sessions" (
    "user_id" BIGINT NOT NULL CHECK ("user_id" IS NULL OR (typeof("user_id") = 'integer')),
    "session_id" TEXT NOT NULL,
    "creation_time" BIGINT NOT NULL CHECK ("creation_time" IS NULL OR (typeof("creation_time") = 'integer')),
    "expiration_time" BIGINT NOT NULL CHECK ("expiration_time" IS NULL OR (typeof("expiration_time") = 'integer')),
    "wrong_attempt" BIGINT NOT NULL DEFAULT (0) CHECK ("wrong_attempt" IS NULL OR (typeof("wrong_attempt") = 'integer' AND "wrong_attempt" BETWEEN -2147483648 AND 2147483647)),
    CONSTRAINT "fk_sessions_user_id" FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE,
    CONSTRAINT "two_factor_sessions_session_id_key" UNIQUE (session_id)
);

CREATE TABLE "two_factor_used_codes" (
    "user_id" BIGINT NOT NULL CHECK ("user_id" IS NULL OR (typeof("user_id") = 'integer')),
    "code_hash" TEXT NOT NULL,
    "used_at" BIGINT NOT NULL CHECK ("used_at" IS NULL OR (typeof("used_at") = 'integer')),
    CONSTRAINT "fk_two_factor_used_codes_user_id" FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE,
    CONSTRAINT "two_factor_used_codes_pkey" PRIMARY KEY (user_id, code_hash)
);

CREATE INDEX idx_two_factor_used_codes_used_at ON two_factor_used_codes (used_at);

CREATE INDEX idx_two_factor_used_codes_user_id ON two_factor_used_codes (user_id);

CREATE TABLE "usage" (
    "user_id" BIGINT NOT NULL CHECK ("user_id" IS NULL OR (typeof("user_id") = 'integer')),
    "storage_consumed" BIGINT NOT NULL CHECK ("storage_consumed" IS NULL OR (typeof("storage_consumed") = 'integer')),
    "photos_file_count" BIGINT CHECK ("photos_file_count" IS NULL OR (typeof("photos_file_count") = 'integer')),
    "locker_file_count" BIGINT CHECK ("locker_file_count" IS NULL OR (typeof("locker_file_count") = 'integer')),
    "file_count_source_version" BIGINT NOT NULL DEFAULT (0) CHECK ("file_count_source_version" IS NULL OR (typeof("file_count_source_version") = 'integer')),
    "file_app_ready" BIGINT NOT NULL DEFAULT (false) CHECK ("file_app_ready" IS NULL OR ("file_app_ready" IN (0, 1))),
    CONSTRAINT "fk_usage_user_id" FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE,
    CONSTRAINT "usage_file_counts_both_ready_or_both_unknown" CHECK (((photos_file_count IS NULL) = (locker_file_count IS NULL))),
    CONSTRAINT "usage_user_id_unique_constraint_index" UNIQUE (user_id)
);

CREATE INDEX usage_user_id_index ON usage (user_id);

CREATE TABLE "user_attachments" (
    "attachment_id" TEXT NOT NULL,
    "user_id" BIGINT NOT NULL CHECK ("user_id" IS NULL OR (typeof("user_id") = 'integer')),
    "attachment_type" TEXT NOT NULL,
    "size" BIGINT NOT NULL CHECK ("size" IS NULL OR (typeof("size") = 'integer')),
    "latest_bucket" TEXT NOT NULL CHECK ("latest_bucket" IS NULL OR ("latest_bucket" IN ('b2-eu-cen', 'scw-eu-fr', 'scw-eu-fr-locked', 'wasabi-eu-central-2', 'wasabi-eu-central-2-v3', 'scw-eu-fr-v3', 'wasabi-eu-central-2-derived', 'b5', 'b6'))),
    "replicated_buckets" TEXT NOT NULL DEFAULT ('[]') CHECK ("replicated_buckets" IS NULL OR (json_valid("replicated_buckets") AND json_type("replicated_buckets") = 'array')),
    "delete_from_buckets" TEXT NOT NULL DEFAULT ('[]') CHECK ("delete_from_buckets" IS NULL OR (json_valid("delete_from_buckets") AND json_type("delete_from_buckets") = 'array')),
    "inflight_rep_buckets" TEXT NOT NULL DEFAULT ('[]') CHECK ("inflight_rep_buckets" IS NULL OR (json_valid("inflight_rep_buckets") AND json_type("inflight_rep_buckets") = 'array')),
    "is_deleted" BIGINT NOT NULL DEFAULT (false) CHECK ("is_deleted" IS NULL OR ("is_deleted" IN (0, 1))),
    "pending_sync" BIGINT NOT NULL DEFAULT (true) CHECK ("pending_sync" IS NULL OR ("pending_sync" IN (0, 1))),
    "sync_locked_till" BIGINT NOT NULL DEFAULT (0) CHECK ("sync_locked_till" IS NULL OR (typeof("sync_locked_till") = 'integer')),
    "created_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("created_at" IS NULL OR (typeof("created_at") = 'integer')),
    "updated_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("updated_at" IS NULL OR (typeof("updated_at") = 'integer')),
    CONSTRAINT "fk_user_attachments_user_id" FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE,
    CONSTRAINT "user_attachments_pkey" PRIMARY KEY (attachment_id)
);

CREATE INDEX idx_user_attachments_pending_sync_locked_till ON user_attachments (is_deleted, sync_locked_till) WHERE (pending_sync = true);

CREATE INDEX idx_user_attachments_lookup ON user_attachments (user_id, attachment_type, is_deleted);

CREATE TABLE "users" (
    "user_id" INTEGER PRIMARY KEY AUTOINCREMENT,
    "name" TEXT,
    "creation_time" BIGINT NOT NULL CHECK ("creation_time" IS NULL OR (typeof("creation_time") = 'integer')),
    "encrypted_email" BLOB CHECK ("encrypted_email" IS NULL OR (typeof("encrypted_email") = 'blob')),
    "email_decryption_nonce" BLOB CHECK ("email_decryption_nonce" IS NULL OR (typeof("email_decryption_nonce") = 'blob')),
    "email_hash" TEXT,
    "is_two_factor_enabled" BIGINT NOT NULL DEFAULT (false) CHECK ("is_two_factor_enabled" IS NULL OR ("is_two_factor_enabled" IN (0, 1))),
    "family_admin_id" BIGINT CHECK ("family_admin_id" IS NULL OR (typeof("family_admin_id") = 'integer')),
    "email_mfa" BIGINT NOT NULL DEFAULT (false) CHECK ("email_mfa" IS NULL OR ("email_mfa" IN (0, 1))),
    "source" TEXT,
    "delete_feedback" ANY CHECK ("delete_feedback" IS NULL OR (json_valid("delete_feedback"))),
    CONSTRAINT "users_email_hash_key" UNIQUE (email_hash)
);

CREATE INDEX users_email_hash_index ON users (email_hash);

CREATE TABLE "webauthn_sessions" (
    "id" TEXT NOT NULL,
    "challenge" TEXT NOT NULL,
    "user_id" BIGINT NOT NULL CHECK ("user_id" IS NULL OR (typeof("user_id") = 'integer')),
    "allowed_credential_ids" TEXT NOT NULL,
    "expires_at" BIGINT NOT NULL CHECK ("expires_at" IS NULL OR (typeof("expires_at") = 'integer')),
    "user_verification_requirement" TEXT NOT NULL,
    "extensions" TEXT NOT NULL,
    "created_at" BIGINT NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000000 AS INTEGER)) CHECK ("created_at" IS NULL OR (typeof("created_at") = 'integer')),
    "rp_id" TEXT,
    CONSTRAINT "fk_webauthn_sessions_user_id" FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE,
    CONSTRAINT "webauthn_sessions_challenge_key" UNIQUE (challenge),
    CONSTRAINT "webauthn_sessions_pkey" PRIMARY KEY (id)
);

CREATE TRIGGER "update_anon_users_updated_at_update" AFTER UPDATE ON "anon_users" WHEN OLD.updated_at >= NEW.updated_at BEGIN
    UPDATE "anon_users" SET updated_at = MAX(OLD.updated_at + 1, CAST(unixepoch('subsec') * 1000000 AS INTEGER)) WHERE "id" = NEW."id";
END;

CREATE TRIGGER "update_authenticator_entity_updated_at_update" AFTER UPDATE ON "authenticator_entity" WHEN OLD.updated_at >= NEW.updated_at BEGIN
    UPDATE "authenticator_entity" SET updated_at = MAX(OLD.updated_at + 1, CAST(unixepoch('subsec') * 1000000 AS INTEGER)) WHERE "id" = NEW."id";
END;

CREATE TRIGGER "trigger_authenticator_key_updated_time_on_authenticator_entity__insert" AFTER INSERT ON "authenticator_entity" BEGIN
    UPDATE authenticator_key SET updated_at = NEW.updated_at WHERE user_id = NEW.user_id AND updated_at < NEW.updated_at;
END;

CREATE TRIGGER "trigger_authenticator_key_updated_time_on_authenticator_entity__update" AFTER UPDATE ON "authenticator_entity" BEGIN
    UPDATE authenticator_key SET updated_at = NEW.updated_at WHERE user_id = NEW.user_id AND updated_at < NEW.updated_at;
END;

CREATE TRIGGER "update_collection_actions_updated_at_update" AFTER UPDATE ON "collection_actions" WHEN OLD.updated_at >= NEW.updated_at BEGIN
    UPDATE "collection_actions" SET updated_at = MAX(OLD.updated_at + 1, CAST(unixepoch('subsec') * 1000000 AS INTEGER)) WHERE "id" = NEW."id";
END;

CREATE TRIGGER "comments_null_cipher_on_delete_insert" AFTER INSERT ON "comments" WHEN NEW.is_deleted AND (NEW.cipher IS NOT NULL OR NEW.nonce IS NOT NULL) BEGIN
    UPDATE "comments" SET cipher = NULL, nonce = NULL WHERE "id" = NEW."id";
END;

CREATE TRIGGER "comments_null_cipher_on_delete_update" AFTER UPDATE ON "comments" WHEN NEW.is_deleted AND (NEW.cipher IS NOT NULL OR NEW.nonce IS NOT NULL) BEGIN
    UPDATE "comments" SET cipher = NULL, nonce = NULL WHERE "id" = NEW."id";
END;

CREATE TRIGGER "update_comments_updated_at_update" AFTER UPDATE ON "comments" WHEN OLD.updated_at >= NEW.updated_at BEGIN
    UPDATE "comments" SET updated_at = MAX(OLD.updated_at + 1, CAST(unixepoch('subsec') * 1000000 AS INTEGER)) WHERE "id" = NEW."id";
END;

CREATE TRIGGER "check_reply_scope_insert" BEFORE INSERT ON "comments" WHEN NEW.parent_comment_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM comments p WHERE p.id = NEW.parent_comment_id AND p.collection_id = NEW.collection_id AND p.file_id IS NEW.file_id) BEGIN
    SELECT RAISE(ABORT, 'Reply must match parent scope');
END;

CREATE TRIGGER "check_reply_scope_update" BEFORE UPDATE ON "comments" WHEN NEW.parent_comment_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM comments p WHERE p.id = NEW.parent_comment_id AND p.collection_id = NEW.collection_id AND p.file_id IS NEW.file_id) BEGIN
    SELECT RAISE(ABORT, 'Reply must match parent scope');
END;

CREATE TRIGGER "soft_delete_reactions_on_comment_delete_update" AFTER UPDATE ON "comments" WHEN NEW.is_deleted AND OLD.is_deleted IS NOT NEW.is_deleted BEGIN
    UPDATE reactions SET is_deleted = true, cipher = NULL, nonce = NULL, updated_at = CAST(unixepoch('subsec') * 1000000 AS INTEGER) WHERE comment_id = NEW.id AND is_deleted = false;
END;

CREATE TRIGGER "trigger_reject_contact_key_update_update" BEFORE UPDATE ON "contact_entity" WHEN NEW.encrypted_key IS NOT OLD.encrypted_key BEGIN
    SELECT RAISE(ABORT, 'contact_entity.encrypted_key is immutable');
END;

CREATE TRIGGER "update_contact_entity_updated_at_update" AFTER UPDATE ON "contact_entity" WHEN OLD.updated_at >= NEW.updated_at BEGIN
    UPDATE "contact_entity" SET updated_at = MAX(OLD.updated_at + 1, CAST(unixepoch('subsec') * 1000000 AS INTEGER)) WHERE "id" = NEW."id";
END;

CREATE TRIGGER "update_discount_coupons_updated_at_update" AFTER UPDATE ON "discount_coupons" WHEN OLD.updated_at >= NEW.updated_at BEGIN
    UPDATE "discount_coupons" SET updated_at = MAX(OLD.updated_at + 1, CAST(unixepoch('subsec') * 1000000 AS INTEGER)) WHERE "rowid" = NEW."rowid";
END;

CREATE TRIGGER "update_entity_data_updated_at_update" AFTER UPDATE ON "entity_data" WHEN OLD.updated_at >= NEW.updated_at BEGIN
    UPDATE "entity_data" SET updated_at = MAX(OLD.updated_at + 1, CAST(unixepoch('subsec') * 1000000 AS INTEGER)) WHERE "id" = NEW."id";
END;

CREATE TRIGGER "update_events_updated_at_update" AFTER UPDATE ON "events" WHEN OLD.updated_at >= NEW.updated_at BEGIN
    UPDATE "events" SET updated_at = MAX(OLD.updated_at + 1, CAST(unixepoch('subsec') * 1000000 AS INTEGER)) WHERE "rowid" = NEW."rowid";
END;

CREATE TRIGGER "update_families_updated_at_update" AFTER UPDATE ON "families" WHEN OLD.updated_at >= NEW.updated_at BEGIN
    UPDATE "families" SET updated_at = MAX(OLD.updated_at + 1, CAST(unixepoch('subsec') * 1000000 AS INTEGER)) WHERE "id" = NEW."id";
END;

CREATE TRIGGER "update_emergency_conctact_updated_at_update" AFTER UPDATE ON "families" WHEN OLD.updated_at >= NEW.updated_at BEGIN
    UPDATE "families" SET updated_at = MAX(OLD.updated_at + 1, CAST(unixepoch('subsec') * 1000000 AS INTEGER)) WHERE "id" = NEW."id";
END;

CREATE TRIGGER "update_emergency_recovery_updated_at_update" AFTER UPDATE ON "families" WHEN OLD.updated_at >= NEW.updated_at BEGIN
    UPDATE "families" SET updated_at = MAX(OLD.updated_at + 1, CAST(unixepoch('subsec') * 1000000 AS INTEGER)) WHERE "id" = NEW."id";
END;

CREATE TRIGGER "check_no_common_entries_insert" BEFORE INSERT ON "file_data" BEGIN
    SELECT RAISE(ABORT, 'Duplicate bucket IDs found') WHERE EXISTS (SELECT value FROM (SELECT NEW.latest_bucket AS value UNION ALL SELECT value FROM json_each(NEW.replicated_buckets) UNION ALL SELECT value FROM json_each(NEW.delete_from_buckets) UNION ALL SELECT value FROM json_each(NEW.inflight_rep_buckets)) WHERE value IS NOT NULL GROUP BY value HAVING COUNT(*) > 1);
    SELECT RAISE(ABORT, 'Invalid bucket ID') WHERE EXISTS (SELECT 1 FROM json_each(NEW.replicated_buckets) WHERE value NOT IN ('b2-eu-cen', 'scw-eu-fr', 'scw-eu-fr-locked', 'wasabi-eu-central-2', 'wasabi-eu-central-2-v3', 'scw-eu-fr-v3', 'wasabi-eu-central-2-derived', 'b5', 'b6') OR type <> 'text');
    SELECT RAISE(ABORT, 'Invalid bucket ID') WHERE EXISTS (SELECT 1 FROM json_each(NEW.delete_from_buckets) WHERE value NOT IN ('b2-eu-cen', 'scw-eu-fr', 'scw-eu-fr-locked', 'wasabi-eu-central-2', 'wasabi-eu-central-2-v3', 'scw-eu-fr-v3', 'wasabi-eu-central-2-derived', 'b5', 'b6') OR type <> 'text');
    SELECT RAISE(ABORT, 'Invalid bucket ID') WHERE EXISTS (SELECT 1 FROM json_each(NEW.inflight_rep_buckets) WHERE value NOT IN ('b2-eu-cen', 'scw-eu-fr', 'scw-eu-fr-locked', 'wasabi-eu-central-2', 'wasabi-eu-central-2-v3', 'scw-eu-fr-v3', 'wasabi-eu-central-2-derived', 'b5', 'b6') OR type <> 'text');
END;

CREATE TRIGGER "check_no_common_entries_update" BEFORE UPDATE ON "file_data" BEGIN
    SELECT RAISE(ABORT, 'Duplicate bucket IDs found') WHERE EXISTS (SELECT value FROM (SELECT NEW.latest_bucket AS value UNION ALL SELECT value FROM json_each(NEW.replicated_buckets) UNION ALL SELECT value FROM json_each(NEW.delete_from_buckets) UNION ALL SELECT value FROM json_each(NEW.inflight_rep_buckets)) WHERE value IS NOT NULL GROUP BY value HAVING COUNT(*) > 1);
    SELECT RAISE(ABORT, 'Invalid bucket ID') WHERE EXISTS (SELECT 1 FROM json_each(NEW.replicated_buckets) WHERE value NOT IN ('b2-eu-cen', 'scw-eu-fr', 'scw-eu-fr-locked', 'wasabi-eu-central-2', 'wasabi-eu-central-2-v3', 'scw-eu-fr-v3', 'wasabi-eu-central-2-derived', 'b5', 'b6') OR type <> 'text');
    SELECT RAISE(ABORT, 'Invalid bucket ID') WHERE EXISTS (SELECT 1 FROM json_each(NEW.delete_from_buckets) WHERE value NOT IN ('b2-eu-cen', 'scw-eu-fr', 'scw-eu-fr-locked', 'wasabi-eu-central-2', 'wasabi-eu-central-2-v3', 'scw-eu-fr-v3', 'wasabi-eu-central-2-derived', 'b5', 'b6') OR type <> 'text');
    SELECT RAISE(ABORT, 'Invalid bucket ID') WHERE EXISTS (SELECT 1 FROM json_each(NEW.inflight_rep_buckets) WHERE value NOT IN ('b2-eu-cen', 'scw-eu-fr', 'scw-eu-fr-locked', 'wasabi-eu-central-2', 'wasabi-eu-central-2-v3', 'scw-eu-fr-v3', 'wasabi-eu-central-2-derived', 'b5', 'b6') OR type <> 'text');
END;

CREATE TRIGGER "update_legacy_kit_updated_at_update" AFTER UPDATE ON "legacy_kit" WHEN OLD.updated_at >= NEW.updated_at BEGIN
    UPDATE "legacy_kit" SET updated_at = MAX(OLD.updated_at + 1, CAST(unixepoch('subsec') * 1000000 AS INTEGER)) WHERE "id" = NEW."id";
END;

CREATE TRIGGER "update_legacy_kit_recovery_session_updated_at_update" AFTER UPDATE ON "legacy_kit_recovery_session" WHEN OLD.updated_at >= NEW.updated_at BEGIN
    UPDATE "legacy_kit_recovery_session" SET updated_at = MAX(OLD.updated_at + 1, CAST(unixepoch('subsec') * 1000000 AS INTEGER)) WHERE "id" = NEW."id";
END;

CREATE TRIGGER "update_object_keys_updated_at_update" AFTER UPDATE ON "object_keys" WHEN OLD.updated_at >= NEW.updated_at BEGIN
    UPDATE "object_keys" SET updated_at = MAX(OLD.updated_at + 1, CAST(unixepoch('subsec') * 1000000 AS INTEGER)) WHERE "file_id" = NEW."file_id" AND "o_type" = NEW."o_type";
END;

CREATE TRIGGER "update_public_collection_tokens_updated_at_update" AFTER UPDATE ON "public_collection_tokens" WHEN OLD.updated_at >= NEW.updated_at BEGIN
    UPDATE "public_collection_tokens" SET updated_at = MAX(OLD.updated_at + 1, CAST(unixepoch('subsec') * 1000000 AS INTEGER)) WHERE "id" = NEW."id";
END;

CREATE TRIGGER "trigger_collection_updation_time_on_collection_tokens_updated_insert" AFTER INSERT ON "public_collection_tokens" BEGIN
    UPDATE collections SET updation_time = NEW.updated_at WHERE collection_id = NEW.collection_id AND updation_time < NEW.updated_at;
END;

CREATE TRIGGER "trigger_collection_updation_time_on_collection_tokens_updated_update" AFTER UPDATE ON "public_collection_tokens" BEGIN
    UPDATE collections SET updation_time = NEW.updated_at WHERE collection_id = NEW.collection_id AND updation_time < NEW.updated_at;
END;

CREATE TRIGGER "update_public_file_tokens_updated_at_update" AFTER UPDATE ON "public_file_tokens" WHEN OLD.updated_at >= NEW.updated_at BEGIN
    UPDATE "public_file_tokens" SET updated_at = MAX(OLD.updated_at + 1, CAST(unixepoch('subsec') * 1000000 AS INTEGER)) WHERE "id" = NEW."id";
END;

CREATE TRIGGER "update_push_tokens_updated_at_update" AFTER UPDATE ON "push_tokens" WHEN OLD.updated_at >= NEW.updated_at BEGIN
    UPDATE "push_tokens" SET updated_at = MAX(OLD.updated_at + 1, CAST(unixepoch('subsec') * 1000000 AS INTEGER)) WHERE "fcm_token" = NEW."fcm_token";
END;

CREATE TRIGGER "update_queue_updated_at_update" AFTER UPDATE ON "queue" WHEN OLD.updated_at >= NEW.updated_at BEGIN
    UPDATE "queue" SET updated_at = MAX(OLD.updated_at + 1, CAST(unixepoch('subsec') * 1000000 AS INTEGER)) WHERE "queue_id" = NEW."queue_id";
END;

CREATE TRIGGER "reactions_null_cipher_on_delete_insert" AFTER INSERT ON "reactions" WHEN NEW.is_deleted AND (NEW.cipher IS NOT NULL OR NEW.nonce IS NOT NULL) BEGIN
    UPDATE "reactions" SET cipher = NULL, nonce = NULL WHERE "id" = NEW."id";
END;

CREATE TRIGGER "reactions_null_cipher_on_delete_update" AFTER UPDATE ON "reactions" WHEN NEW.is_deleted AND (NEW.cipher IS NOT NULL OR NEW.nonce IS NOT NULL) BEGIN
    UPDATE "reactions" SET cipher = NULL, nonce = NULL WHERE "id" = NEW."id";
END;

CREATE TRIGGER "update_reactions_updated_at_update" AFTER UPDATE ON "reactions" WHEN OLD.updated_at >= NEW.updated_at BEGIN
    UPDATE "reactions" SET updated_at = MAX(OLD.updated_at + 1, CAST(unixepoch('subsec') * 1000000 AS INTEGER)) WHERE "id" = NEW."id";
END;

CREATE TRIGGER "check_reaction_target_insert" BEFORE INSERT ON "reactions" WHEN NEW.comment_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM comments c WHERE c.id = NEW.comment_id AND c.collection_id = NEW.collection_id) BEGIN
    SELECT RAISE(ABORT, 'Reaction collection_id must match existing comment');
END;

CREATE TRIGGER "check_reaction_target_update" BEFORE UPDATE ON "reactions" WHEN NEW.comment_id IS NOT NULL AND NOT EXISTS (SELECT 1 FROM comments c WHERE c.id = NEW.comment_id AND c.collection_id = NEW.collection_id) BEGIN
    SELECT RAISE(ABORT, 'Reaction collection_id must match existing comment');
END;

CREATE TRIGGER "update_remote_store_updated_at_update" AFTER UPDATE ON "remote_store" WHEN OLD.updated_at >= NEW.updated_at BEGIN
    UPDATE "remote_store" SET updated_at = MAX(OLD.updated_at + 1, CAST(unixepoch('subsec') * 1000000 AS INTEGER)) WHERE "user_id" = NEW."user_id" AND "key_name" = NEW."key_name";
END;

CREATE TRIGGER "update_space_browser_sessions_updated_at_update" AFTER UPDATE ON "space_browser_sessions" WHEN OLD.updated_at >= NEW.updated_at BEGIN
    UPDATE "space_browser_sessions" SET updated_at = MAX(OLD.updated_at + 1, CAST(unixepoch('subsec') * 1000000 AS INTEGER)) WHERE "token_hash" = NEW."token_hash";
END;

CREATE TRIGGER "update_space_friend_requests_updated_at_update" AFTER UPDATE ON "space_friend_requests" WHEN OLD.updated_at >= NEW.updated_at BEGIN
    UPDATE "space_friend_requests" SET updated_at = MAX(OLD.updated_at + 1, CAST(unixepoch('subsec') * 1000000 AS INTEGER)) WHERE "request_id" = NEW."request_id";
END;

CREATE TRIGGER "update_space_friend_shares_updated_at_update" AFTER UPDATE ON "space_friend_shares" WHEN OLD.updated_at >= NEW.updated_at BEGIN
    UPDATE "space_friend_shares" SET updated_at = MAX(OLD.updated_at + 1, CAST(unixepoch('subsec') * 1000000 AS INTEGER)) WHERE "space_id" = NEW."space_id" AND "friend_space_id" = NEW."friend_space_id";
END;

CREATE TRIGGER "update_space_links_updated_at_update" AFTER UPDATE ON "space_links" WHEN OLD.updated_at >= NEW.updated_at BEGIN
    UPDATE "space_links" SET updated_at = MAX(OLD.updated_at + 1, CAST(unixepoch('subsec') * 1000000 AS INTEGER)) WHERE "link_id" = NEW."link_id";
END;

CREATE TRIGGER "space_messages_null_cipher_on_delete_insert" AFTER INSERT ON "space_messages" WHEN NEW.is_deleted AND (NEW.message_cipher IS NOT NULL OR NEW.sender_encrypted_message_key IS NOT NULL OR NEW.recipient_encrypted_message_key IS NOT NULL OR NEW.recipient_liked_at IS NOT NULL OR NEW.sender_encrypted_reaction IS NOT NULL OR NEW.recipient_encrypted_reaction IS NOT NULL) BEGIN
    UPDATE "space_messages" SET message_cipher = NULL, sender_encrypted_message_key = NULL, recipient_encrypted_message_key = NULL, recipient_liked_at = NULL, sender_encrypted_reaction = NULL, recipient_encrypted_reaction = NULL WHERE "message_id" = NEW."message_id";
END;

CREATE TRIGGER "space_messages_null_cipher_on_delete_update" AFTER UPDATE ON "space_messages" WHEN NEW.is_deleted AND (NEW.message_cipher IS NOT NULL OR NEW.sender_encrypted_message_key IS NOT NULL OR NEW.recipient_encrypted_message_key IS NOT NULL OR NEW.recipient_liked_at IS NOT NULL OR NEW.sender_encrypted_reaction IS NOT NULL OR NEW.recipient_encrypted_reaction IS NOT NULL) BEGIN
    UPDATE "space_messages" SET message_cipher = NULL, sender_encrypted_message_key = NULL, recipient_encrypted_message_key = NULL, recipient_liked_at = NULL, sender_encrypted_reaction = NULL, recipient_encrypted_reaction = NULL WHERE "message_id" = NEW."message_id";
END;

CREATE TRIGGER "update_space_messages_updated_at_update" AFTER UPDATE ON "space_messages" WHEN OLD.updated_at >= NEW.updated_at BEGIN
    UPDATE "space_messages" SET updated_at = MAX(OLD.updated_at + 1, CAST(unixepoch('subsec') * 1000000 AS INTEGER)) WHERE "message_id" = NEW."message_id";
END;

CREATE TRIGGER "update_space_notification_read_markers_updated_at_update" AFTER UPDATE ON "space_notification_read_markers" WHEN OLD.updated_at >= NEW.updated_at BEGIN
    UPDATE "space_notification_read_markers" SET updated_at = MAX(OLD.updated_at + 1, CAST(unixepoch('subsec') * 1000000 AS INTEGER)) WHERE "viewer_space_id" = NEW."viewer_space_id" AND "friend_space_id" = NEW."friend_space_id";
END;

CREATE TRIGGER "update_space_posts_updated_at_update" AFTER UPDATE ON "space_posts" WHEN OLD.updated_at >= NEW.updated_at BEGIN
    UPDATE "space_posts" SET updated_at = MAX(OLD.updated_at + 1, CAST(unixepoch('subsec') * 1000000 AS INTEGER)) WHERE "post_id" = NEW."post_id";
END;

CREATE TRIGGER "update_space_profile_assets_updated_at_update" AFTER UPDATE ON "space_profile_assets" WHEN OLD.updated_at >= NEW.updated_at BEGIN
    UPDATE "space_profile_assets" SET updated_at = MAX(OLD.updated_at + 1, CAST(unixepoch('subsec') * 1000000 AS INTEGER)) WHERE "space_id" = NEW."space_id" AND "asset_type" = NEW."asset_type";
END;

CREATE TRIGGER "update_space_web_push_subscriptions_updated_at_update" AFTER UPDATE ON "space_web_push_subscriptions" WHEN OLD.updated_at >= NEW.updated_at BEGIN
    UPDATE "space_web_push_subscriptions" SET updated_at = MAX(OLD.updated_at + 1, CAST(unixepoch('subsec') * 1000000 AS INTEGER)) WHERE "target_id" = NEW."target_id";
END;

CREATE TRIGGER "update_spaces_updated_at_update" AFTER UPDATE ON "spaces" WHEN OLD.updated_at >= NEW.updated_at BEGIN
    UPDATE "spaces" SET updated_at = MAX(OLD.updated_at + 1, CAST(unixepoch('subsec') * 1000000 AS INTEGER)) WHERE "space_id" = NEW."space_id";
END;

CREATE TRIGGER "update_srp_auth_updated_at_update" AFTER UPDATE ON "srp_auth" WHEN OLD.updated_at >= NEW.updated_at BEGIN
    UPDATE "srp_auth" SET updated_at = MAX(OLD.updated_at + 1, CAST(unixepoch('subsec') * 1000000 AS INTEGER)) WHERE "user_id" = NEW."user_id";
END;

CREATE TRIGGER "update_srp_sessions_updated_at_update" AFTER UPDATE ON "srp_sessions" WHEN OLD.updated_at >= NEW.updated_at BEGIN
    UPDATE "srp_sessions" SET updated_at = MAX(OLD.updated_at + 1, CAST(unixepoch('subsec') * 1000000 AS INTEGER)) WHERE "id" = NEW."id";
END;

CREATE TRIGGER "sync_token_hash_insert" BEFORE INSERT ON "tokens" WHEN NEW.token IS NOT NULL BEGIN
    SELECT RAISE(ABORT, 'tokens require application-computed token_hash and NULL token');
END;

CREATE TRIGGER "sync_token_hash_update" BEFORE UPDATE ON "tokens" WHEN NEW.token IS NOT NULL BEGIN
    SELECT RAISE(ABORT, 'tokens require application-computed token_hash and NULL token');
END;

CREATE TRIGGER "update_trash_updated_at_update" AFTER UPDATE ON "trash" WHEN OLD.updated_at >= NEW.updated_at BEGIN
    UPDATE "trash" SET updated_at = MAX(OLD.updated_at + 1, CAST(unixepoch('subsec') * 1000000 AS INTEGER)) WHERE "file_id" = NEW."file_id";
END;

CREATE TRIGGER "update_two_factor_recovery_updated_at_update" AFTER UPDATE ON "two_factor_recovery" WHEN OLD.updated_at >= NEW.updated_at BEGIN
    UPDATE "two_factor_recovery" SET updated_at = MAX(OLD.updated_at + 1, CAST(unixepoch('subsec') * 1000000 AS INTEGER)) WHERE "user_id" = NEW."user_id";
END;

CREATE TRIGGER "check_no_common_attachment_buckets_insert" BEFORE INSERT ON "user_attachments" BEGIN
    SELECT RAISE(ABORT, 'Duplicate bucket IDs found') WHERE EXISTS (SELECT value FROM (SELECT NEW.latest_bucket AS value UNION ALL SELECT value FROM json_each(NEW.replicated_buckets) UNION ALL SELECT value FROM json_each(NEW.delete_from_buckets) UNION ALL SELECT value FROM json_each(NEW.inflight_rep_buckets)) WHERE value IS NOT NULL GROUP BY value HAVING COUNT(*) > 1);
    SELECT RAISE(ABORT, 'Invalid bucket ID') WHERE EXISTS (SELECT 1 FROM json_each(NEW.replicated_buckets) WHERE value NOT IN ('b2-eu-cen', 'scw-eu-fr', 'scw-eu-fr-locked', 'wasabi-eu-central-2', 'wasabi-eu-central-2-v3', 'scw-eu-fr-v3', 'wasabi-eu-central-2-derived', 'b5', 'b6') OR type <> 'text');
    SELECT RAISE(ABORT, 'Invalid bucket ID') WHERE EXISTS (SELECT 1 FROM json_each(NEW.delete_from_buckets) WHERE value NOT IN ('b2-eu-cen', 'scw-eu-fr', 'scw-eu-fr-locked', 'wasabi-eu-central-2', 'wasabi-eu-central-2-v3', 'scw-eu-fr-v3', 'wasabi-eu-central-2-derived', 'b5', 'b6') OR type <> 'text');
    SELECT RAISE(ABORT, 'Invalid bucket ID') WHERE EXISTS (SELECT 1 FROM json_each(NEW.inflight_rep_buckets) WHERE value NOT IN ('b2-eu-cen', 'scw-eu-fr', 'scw-eu-fr-locked', 'wasabi-eu-central-2', 'wasabi-eu-central-2-v3', 'scw-eu-fr-v3', 'wasabi-eu-central-2-derived', 'b5', 'b6') OR type <> 'text');
END;

CREATE TRIGGER "check_no_common_attachment_buckets_update" BEFORE UPDATE ON "user_attachments" BEGIN
    SELECT RAISE(ABORT, 'Duplicate bucket IDs found') WHERE EXISTS (SELECT value FROM (SELECT NEW.latest_bucket AS value UNION ALL SELECT value FROM json_each(NEW.replicated_buckets) UNION ALL SELECT value FROM json_each(NEW.delete_from_buckets) UNION ALL SELECT value FROM json_each(NEW.inflight_rep_buckets)) WHERE value IS NOT NULL GROUP BY value HAVING COUNT(*) > 1);
    SELECT RAISE(ABORT, 'Invalid bucket ID') WHERE EXISTS (SELECT 1 FROM json_each(NEW.replicated_buckets) WHERE value NOT IN ('b2-eu-cen', 'scw-eu-fr', 'scw-eu-fr-locked', 'wasabi-eu-central-2', 'wasabi-eu-central-2-v3', 'scw-eu-fr-v3', 'wasabi-eu-central-2-derived', 'b5', 'b6') OR type <> 'text');
    SELECT RAISE(ABORT, 'Invalid bucket ID') WHERE EXISTS (SELECT 1 FROM json_each(NEW.delete_from_buckets) WHERE value NOT IN ('b2-eu-cen', 'scw-eu-fr', 'scw-eu-fr-locked', 'wasabi-eu-central-2', 'wasabi-eu-central-2-v3', 'scw-eu-fr-v3', 'wasabi-eu-central-2-derived', 'b5', 'b6') OR type <> 'text');
    SELECT RAISE(ABORT, 'Invalid bucket ID') WHERE EXISTS (SELECT 1 FROM json_each(NEW.inflight_rep_buckets) WHERE value NOT IN ('b2-eu-cen', 'scw-eu-fr', 'scw-eu-fr-locked', 'wasabi-eu-central-2', 'wasabi-eu-central-2-v3', 'scw-eu-fr-v3', 'wasabi-eu-central-2-derived', 'b5', 'b6') OR type <> 'text');
END;

CREATE TRIGGER "update_user_attachments_updated_at_update" AFTER UPDATE ON "user_attachments" WHEN OLD.updated_at >= NEW.updated_at BEGIN
    UPDATE "user_attachments" SET updated_at = MAX(OLD.updated_at + 1, CAST(unixepoch('subsec') * 1000000 AS INTEGER)) WHERE "attachment_id" = NEW."attachment_id";
END;

-- Match PostgreSQL identity floors and prevent externally visible ID reuse.
INSERT INTO sqlite_sequence(name, seq) VALUES ('users',1580559962386437), ('collections',1580559962386437), ('files',9999999);

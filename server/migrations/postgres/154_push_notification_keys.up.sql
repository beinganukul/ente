-- Upstream 152 follows this fork's already-applied migrations 152 and 153.
ALTER TABLE push_tokens ADD COLUMN notification_public_key BYTEA
    CHECK (notification_public_key IS NULL OR octet_length(notification_public_key) = 1216);

ALTER TABLE push_tokens ADD COLUMN notification_public_key BLOB
    CHECK (notification_public_key IS NULL OR (typeof(notification_public_key) = 'blob' AND length(notification_public_key) = 1216));

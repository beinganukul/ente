ALTER TABLE temp_objects ADD COLUMN cleanup_claim TEXT;
ALTER TABLE temp_objects ADD COLUMN cleanup_claim_until BIGINT NOT NULL DEFAULT 0;

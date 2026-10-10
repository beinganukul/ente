ALTER TABLE temp_objects ADD COLUMN cleanup_claim TEXT;
ALTER TABLE temp_objects ADD COLUMN cleanup_claim_until INTEGER NOT NULL DEFAULT 0;

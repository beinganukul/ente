-- Match PostgreSQL enum arrays, including NULL elements and duplicate NULLs.

DROP TRIGGER "check_no_common_entries_insert";

CREATE TRIGGER "check_no_common_entries_insert" BEFORE INSERT ON "file_data" BEGIN
    SELECT RAISE(ABORT, 'Duplicate bucket IDs found') WHERE EXISTS (SELECT value FROM (SELECT NEW.latest_bucket AS value UNION ALL SELECT value FROM json_each(NEW.replicated_buckets) UNION ALL SELECT value FROM json_each(NEW.delete_from_buckets) UNION ALL SELECT value FROM json_each(NEW.inflight_rep_buckets)) GROUP BY value HAVING COUNT(*) > 1);
    SELECT RAISE(ABORT, 'Invalid bucket ID') WHERE EXISTS (SELECT 1 FROM json_each(NEW.replicated_buckets) WHERE value NOT IN ('b2-eu-cen', 'scw-eu-fr', 'scw-eu-fr-locked', 'wasabi-eu-central-2', 'wasabi-eu-central-2-v3', 'scw-eu-fr-v3', 'wasabi-eu-central-2-derived', 'b5', 'b6') OR type NOT IN ('text','null'));
    SELECT RAISE(ABORT, 'Invalid bucket ID') WHERE EXISTS (SELECT 1 FROM json_each(NEW.delete_from_buckets) WHERE value NOT IN ('b2-eu-cen', 'scw-eu-fr', 'scw-eu-fr-locked', 'wasabi-eu-central-2', 'wasabi-eu-central-2-v3', 'scw-eu-fr-v3', 'wasabi-eu-central-2-derived', 'b5', 'b6') OR type NOT IN ('text','null'));
    SELECT RAISE(ABORT, 'Invalid bucket ID') WHERE EXISTS (SELECT 1 FROM json_each(NEW.inflight_rep_buckets) WHERE value NOT IN ('b2-eu-cen', 'scw-eu-fr', 'scw-eu-fr-locked', 'wasabi-eu-central-2', 'wasabi-eu-central-2-v3', 'scw-eu-fr-v3', 'wasabi-eu-central-2-derived', 'b5', 'b6') OR type NOT IN ('text','null'));
END;

DROP TRIGGER "check_no_common_entries_update";

CREATE TRIGGER "check_no_common_entries_update" BEFORE UPDATE ON "file_data" BEGIN
    SELECT RAISE(ABORT, 'Duplicate bucket IDs found') WHERE EXISTS (SELECT value FROM (SELECT NEW.latest_bucket AS value UNION ALL SELECT value FROM json_each(NEW.replicated_buckets) UNION ALL SELECT value FROM json_each(NEW.delete_from_buckets) UNION ALL SELECT value FROM json_each(NEW.inflight_rep_buckets)) GROUP BY value HAVING COUNT(*) > 1);
    SELECT RAISE(ABORT, 'Invalid bucket ID') WHERE EXISTS (SELECT 1 FROM json_each(NEW.replicated_buckets) WHERE value NOT IN ('b2-eu-cen', 'scw-eu-fr', 'scw-eu-fr-locked', 'wasabi-eu-central-2', 'wasabi-eu-central-2-v3', 'scw-eu-fr-v3', 'wasabi-eu-central-2-derived', 'b5', 'b6') OR type NOT IN ('text','null'));
    SELECT RAISE(ABORT, 'Invalid bucket ID') WHERE EXISTS (SELECT 1 FROM json_each(NEW.delete_from_buckets) WHERE value NOT IN ('b2-eu-cen', 'scw-eu-fr', 'scw-eu-fr-locked', 'wasabi-eu-central-2', 'wasabi-eu-central-2-v3', 'scw-eu-fr-v3', 'wasabi-eu-central-2-derived', 'b5', 'b6') OR type NOT IN ('text','null'));
    SELECT RAISE(ABORT, 'Invalid bucket ID') WHERE EXISTS (SELECT 1 FROM json_each(NEW.inflight_rep_buckets) WHERE value NOT IN ('b2-eu-cen', 'scw-eu-fr', 'scw-eu-fr-locked', 'wasabi-eu-central-2', 'wasabi-eu-central-2-v3', 'scw-eu-fr-v3', 'wasabi-eu-central-2-derived', 'b5', 'b6') OR type NOT IN ('text','null'));
END;

DROP TRIGGER "check_no_common_attachment_buckets_insert";

CREATE TRIGGER "check_no_common_attachment_buckets_insert" BEFORE INSERT ON "user_attachments" BEGIN
    SELECT RAISE(ABORT, 'Duplicate bucket IDs found') WHERE EXISTS (SELECT value FROM (SELECT NEW.latest_bucket AS value UNION ALL SELECT value FROM json_each(NEW.replicated_buckets) UNION ALL SELECT value FROM json_each(NEW.delete_from_buckets) UNION ALL SELECT value FROM json_each(NEW.inflight_rep_buckets)) GROUP BY value HAVING COUNT(*) > 1);
    SELECT RAISE(ABORT, 'Invalid bucket ID') WHERE EXISTS (SELECT 1 FROM json_each(NEW.replicated_buckets) WHERE value NOT IN ('b2-eu-cen', 'scw-eu-fr', 'scw-eu-fr-locked', 'wasabi-eu-central-2', 'wasabi-eu-central-2-v3', 'scw-eu-fr-v3', 'wasabi-eu-central-2-derived', 'b5', 'b6') OR type NOT IN ('text','null'));
    SELECT RAISE(ABORT, 'Invalid bucket ID') WHERE EXISTS (SELECT 1 FROM json_each(NEW.delete_from_buckets) WHERE value NOT IN ('b2-eu-cen', 'scw-eu-fr', 'scw-eu-fr-locked', 'wasabi-eu-central-2', 'wasabi-eu-central-2-v3', 'scw-eu-fr-v3', 'wasabi-eu-central-2-derived', 'b5', 'b6') OR type NOT IN ('text','null'));
    SELECT RAISE(ABORT, 'Invalid bucket ID') WHERE EXISTS (SELECT 1 FROM json_each(NEW.inflight_rep_buckets) WHERE value NOT IN ('b2-eu-cen', 'scw-eu-fr', 'scw-eu-fr-locked', 'wasabi-eu-central-2', 'wasabi-eu-central-2-v3', 'scw-eu-fr-v3', 'wasabi-eu-central-2-derived', 'b5', 'b6') OR type NOT IN ('text','null'));
END;

DROP TRIGGER "check_no_common_attachment_buckets_update";

CREATE TRIGGER "check_no_common_attachment_buckets_update" BEFORE UPDATE ON "user_attachments" BEGIN
    SELECT RAISE(ABORT, 'Duplicate bucket IDs found') WHERE EXISTS (SELECT value FROM (SELECT NEW.latest_bucket AS value UNION ALL SELECT value FROM json_each(NEW.replicated_buckets) UNION ALL SELECT value FROM json_each(NEW.delete_from_buckets) UNION ALL SELECT value FROM json_each(NEW.inflight_rep_buckets)) GROUP BY value HAVING COUNT(*) > 1);
    SELECT RAISE(ABORT, 'Invalid bucket ID') WHERE EXISTS (SELECT 1 FROM json_each(NEW.replicated_buckets) WHERE value NOT IN ('b2-eu-cen', 'scw-eu-fr', 'scw-eu-fr-locked', 'wasabi-eu-central-2', 'wasabi-eu-central-2-v3', 'scw-eu-fr-v3', 'wasabi-eu-central-2-derived', 'b5', 'b6') OR type NOT IN ('text','null'));
    SELECT RAISE(ABORT, 'Invalid bucket ID') WHERE EXISTS (SELECT 1 FROM json_each(NEW.delete_from_buckets) WHERE value NOT IN ('b2-eu-cen', 'scw-eu-fr', 'scw-eu-fr-locked', 'wasabi-eu-central-2', 'wasabi-eu-central-2-v3', 'scw-eu-fr-v3', 'wasabi-eu-central-2-derived', 'b5', 'b6') OR type NOT IN ('text','null'));
    SELECT RAISE(ABORT, 'Invalid bucket ID') WHERE EXISTS (SELECT 1 FROM json_each(NEW.inflight_rep_buckets) WHERE value NOT IN ('b2-eu-cen', 'scw-eu-fr', 'scw-eu-fr-locked', 'wasabi-eu-central-2', 'wasabi-eu-central-2-v3', 'scw-eu-fr-v3', 'wasabi-eu-central-2-derived', 'b5', 'b6') OR type NOT IN ('text','null'));
END;

CREATE TRIGGER "validate_object_keys_datacenters_insert" BEFORE INSERT ON "object_keys" BEGIN
    SELECT RAISE(ABORT, 'Invalid datacenter ID') WHERE EXISTS (
        SELECT 1 FROM json_each(NEW.datacenters)
        WHERE value NOT IN ('b2-eu-cen', 'scw-eu-fr', 'scw-eu-fr-locked', 'wasabi-eu-central-2', 'wasabi-eu-central-2-v3', 'scw-eu-fr-v3', 'wasabi-eu-central-2-derived', 'b5', 'b6') OR type NOT IN ('text','null')
    );
END;

CREATE TRIGGER "validate_object_keys_datacenters_update" BEFORE UPDATE ON "object_keys" BEGIN
    SELECT RAISE(ABORT, 'Invalid datacenter ID') WHERE EXISTS (
        SELECT 1 FROM json_each(NEW.datacenters)
        WHERE value NOT IN ('b2-eu-cen', 'scw-eu-fr', 'scw-eu-fr-locked', 'wasabi-eu-central-2', 'wasabi-eu-central-2-v3', 'scw-eu-fr-v3', 'wasabi-eu-central-2-derived', 'b5', 'b6') OR type NOT IN ('text','null')
    );
END;

CREATE TRIGGER "validate_embeddings_datacenters_insert" BEFORE INSERT ON "embeddings" BEGIN
    SELECT RAISE(ABORT, 'Invalid datacenter ID') WHERE EXISTS (
        SELECT 1 FROM json_each(NEW.datacenters)
        WHERE value NOT IN ('b2-eu-cen', 'scw-eu-fr', 'scw-eu-fr-locked', 'wasabi-eu-central-2', 'wasabi-eu-central-2-v3', 'scw-eu-fr-v3', 'wasabi-eu-central-2-derived', 'b5', 'b6') OR type NOT IN ('text','null')
    );
END;

CREATE TRIGGER "validate_embeddings_datacenters_update" BEFORE UPDATE ON "embeddings" BEGIN
    SELECT RAISE(ABORT, 'Invalid datacenter ID') WHERE EXISTS (
        SELECT 1 FROM json_each(NEW.datacenters)
        WHERE value NOT IN ('b2-eu-cen', 'scw-eu-fr', 'scw-eu-fr-locked', 'wasabi-eu-central-2', 'wasabi-eu-central-2-v3', 'scw-eu-fr-v3', 'wasabi-eu-central-2-derived', 'b5', 'b6') OR type NOT IN ('text','null')
    );
END;

-- QQ remains a custom user attribute, but its value must identify one user.
-- A trigger is used instead of a partial unique index because the QQ attribute
-- ID is deployment-specific; its stable key is "qq".
CREATE OR REPLACE FUNCTION enforce_qq_attribute_value_unique()
RETURNS TRIGGER AS $$
DECLARE
    qq_attribute_id BIGINT;
BEGIN
    SELECT id INTO qq_attribute_id
    FROM user_attribute_definitions
    WHERE key = 'qq' AND deleted_at IS NULL
    LIMIT 1;

    IF qq_attribute_id IS NULL OR NEW.attribute_id <> qq_attribute_id OR BTRIM(NEW.value) = '' THEN
        RETURN NEW;
    END IF;

    -- A trigger-side existence check alone is not safe at READ COMMITTED:
    -- concurrent inserts for the same QQ cannot see each other before commit.
    -- Serialize only that normalized QQ for the lifetime of this transaction.
    PERFORM pg_advisory_xact_lock(hashtext(LOWER(BTRIM(NEW.value))));

    IF EXISTS (
        SELECT 1
        FROM user_attribute_values
        WHERE attribute_id = qq_attribute_id
          AND user_id <> NEW.user_id
          AND LOWER(BTRIM(value)) = LOWER(BTRIM(NEW.value))
    ) THEN
        RAISE EXCEPTION 'qq value is already bound' USING ERRCODE = 'unique_violation';
    END IF;

    NEW.value := BTRIM(NEW.value);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_enforce_qq_attribute_value_unique ON user_attribute_values;
CREATE TRIGGER trg_enforce_qq_attribute_value_unique
BEFORE INSERT OR UPDATE OF attribute_id, value ON user_attribute_values
FOR EACH ROW EXECUTE FUNCTION enforce_qq_attribute_value_unique();

CREATE INDEX IF NOT EXISTS idx_user_attribute_values_qq_lookup
ON user_attribute_values (attribute_id, LOWER(BTRIM(value)))
WHERE BTRIM(value) <> '';

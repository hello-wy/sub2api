-- 触发器必须使用状态表的持久化时区，而非当前数据库会话时区。
-- 后台汇总与网关写入可能来自时区不同的连接池会话。

CREATE OR REPLACE FUNCTION invalidate_group_usage_rollup_state()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    affected_date DATE;
    published_before DATE;
    configured_timezone TEXT;
BEGIN
    SELECT closed_before, timezone_name
    INTO published_before, configured_timezone
    FROM usage_group_rollup_state
    WHERE id = 1
    FOR UPDATE;

    IF TG_OP = 'DELETE' THEN
        affected_date := (OLD.created_at AT TIME ZONE configured_timezone)::date;
    ELSIF OLD.group_id IS NULL THEN
        affected_date := (NEW.created_at AT TIME ZONE configured_timezone)::date;
    ELSIF NEW.group_id IS NULL THEN
        affected_date := (OLD.created_at AT TIME ZONE configured_timezone)::date;
    ELSE
        affected_date := LEAST(
            (OLD.created_at AT TIME ZONE configured_timezone)::date,
            (NEW.created_at AT TIME ZONE configured_timezone)::date
        );
    END IF;

    IF published_before > affected_date THEN
        UPDATE usage_group_rollup_state
        SET closed_before = LEAST(closed_before, affected_date),
            updated_at = NOW()
        WHERE id = 1;
    END IF;

    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END;
$$;

CREATE OR REPLACE FUNCTION invalidate_group_usage_rollup_state_after_insert()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
DECLARE
    affected_date DATE;
    published_before DATE;
    configured_timezone TEXT;
BEGIN
    SELECT closed_before, timezone_name
    INTO published_before, configured_timezone
    FROM usage_group_rollup_state
    WHERE id = 1
    FOR KEY SHARE;

    SELECT MIN((created_at AT TIME ZONE configured_timezone)::date)
    INTO affected_date
    FROM inserted_usage_logs
    WHERE group_id IS NOT NULL;

    IF affected_date IS NULL THEN
        RETURN NULL;
    END IF;

    IF published_before > affected_date THEN
        UPDATE usage_group_rollup_state
        SET closed_before = LEAST(closed_before, affected_date),
            updated_at = NOW()
        WHERE id = 1;
    END IF;

    RETURN NULL;
END;
$$;

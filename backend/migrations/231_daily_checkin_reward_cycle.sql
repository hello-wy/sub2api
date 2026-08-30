-- Keep the streak counter cumulative while cycling reward milestones every 30 days.
INSERT INTO settings (key, value)
VALUES ('daily_checkin_cycle_days', '30')
ON CONFLICT (key) DO NOTHING;

-- Earlier versions calculated streaks from only the latest 40 records. Rebuild
-- every successful record from its uninterrupted run of check-in dates.
WITH ordered AS (
    SELECT
        id,
        user_id,
        checkin_date,
        checkin_date - (ROW_NUMBER() OVER (
            PARTITION BY user_id
            ORDER BY checkin_date
        ))::integer AS run_key
    FROM daily_checkin_records
    WHERE status = 'success'
), numbered AS (
    SELECT
        id,
        ROW_NUMBER() OVER (
            PARTITION BY user_id, run_key
            ORDER BY checkin_date
        )::integer AS streak_days
    FROM ordered
)
UPDATE daily_checkin_records AS record
SET
    streak_days = numbered.streak_days,
    updated_at = NOW()
FROM numbered
WHERE record.id = numbered.id
  AND record.streak_days IS DISTINCT FROM numbered.streak_days;

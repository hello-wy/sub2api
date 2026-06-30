ALTER TABLE daily_checkin_records
    ADD COLUMN IF NOT EXISTS status VARCHAR(30) NOT NULL DEFAULT 'success';

CREATE INDEX IF NOT EXISTS dailycheckinrecord_status
    ON daily_checkin_records(status);

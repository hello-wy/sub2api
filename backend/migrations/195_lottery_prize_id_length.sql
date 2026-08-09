-- Older lottery settings generated IDs as "prize-" plus a full UUID (42
-- characters). Keep those already-saved pools drawable while new IDs are
-- generated at 32 characters by the application.
ALTER TABLE lottery_draws
    ALTER COLUMN prize_id TYPE VARCHAR(64);

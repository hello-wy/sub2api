INSERT INTO settings (key, value, updated_at)
VALUES ('welfare_leaderboard_visible_user_ids', '[]', NOW())
ON CONFLICT (key) DO NOTHING;

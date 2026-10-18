-- Preserve the previously excluded account as editable data, keyed by user ID.
INSERT INTO settings (key, value, updated_at)
SELECT 'welfare_excluded_user_ids', COALESCE(jsonb_agg(id ORDER BY id), '[]'::jsonb)::text, NOW()
FROM users
WHERE LOWER(email) = '2281137167@qq.com' AND deleted_at IS NULL
ON CONFLICT (key) DO NOTHING;

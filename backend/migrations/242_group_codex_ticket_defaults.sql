ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS codex_ticket_defaults JSONB NOT NULL DEFAULT '{}'::jsonb;

COMMENT ON COLUMN groups.codex_ticket_defaults IS
    'STATE defaults for newly imported OpenAI OAuth accounts; enabled, ticket_plan (pro/team), model. Existing account settings are not overwritten.';

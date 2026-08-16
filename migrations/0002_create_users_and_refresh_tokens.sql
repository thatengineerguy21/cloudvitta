-- 0002_create_users_and_refresh_tokens.sql
-- Strategy: Forward-Only Migration (No rollback/down migration script, ADR 0028).
-- Purpose: Creates the users table and refresh_tokens table with family tracking.

CREATE TABLE IF NOT EXISTS users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS refresh_tokens (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    -- ON DELETE CASCADE is deliberate forward-looking hygiene for future account deletion workflows.
    user_id      UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    family_id    UUID NOT NULL,
    token_hash   TEXT NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at   TIMESTAMPTZ NOT NULL,
    revoked_at   TIMESTAMPTZ,
    replaced_by  UUID REFERENCES refresh_tokens(id)
);

CREATE INDEX IF NOT EXISTS idx_refresh_tokens_token_hash ON refresh_tokens (token_hash);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_family_id ON refresh_tokens (family_id);

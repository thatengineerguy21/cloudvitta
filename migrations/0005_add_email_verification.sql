-- 0005_add_email_verification.sql
-- Add email verification support to users table and create verification_tokens table.

-- Step 1: Add email_verified flag to users.
-- All existing users are marked verified (they registered before this feature existed).
ALTER TABLE users
    ADD COLUMN email_verified BOOLEAN NOT NULL DEFAULT true;

-- Step 2: For new rows going forward, the application will explicitly set email_verified = false.
-- No DEFAULT change needed; the INSERT query will supply the value.

-- Step 3: Create verification_tokens table.
CREATE TABLE IF NOT EXISTS verification_tokens (
    id           UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID          NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash   TEXT          NOT NULL,
    purpose      TEXT          NOT NULL DEFAULT 'email_verification',
    created_at   TIMESTAMPTZ   NOT NULL DEFAULT now(),
    expires_at   TIMESTAMPTZ   NOT NULL,
    consumed_at  TIMESTAMPTZ
);

-- Index: look up tokens by hash (the primary query path).
CREATE UNIQUE INDEX IF NOT EXISTS idx_verification_tokens_hash ON verification_tokens (token_hash);

-- Index: find active tokens by user for rate-limiting / cleanup.
CREATE INDEX IF NOT EXISTS idx_verification_tokens_user ON verification_tokens (user_id, purpose, created_at DESC);

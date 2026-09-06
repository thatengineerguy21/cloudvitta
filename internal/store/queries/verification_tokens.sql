-- name: InsertVerificationToken :one
INSERT INTO verification_tokens (user_id, token_hash, purpose, expires_at)
VALUES ($1, $2, $3, $4)
RETURNING id, user_id, token_hash, purpose, created_at, expires_at, consumed_at;

-- name: GetVerificationTokenByHash :one
SELECT id, user_id, token_hash, purpose, created_at, expires_at, consumed_at
FROM verification_tokens
WHERE token_hash = $1;

-- name: ConsumeVerificationToken :exec
UPDATE verification_tokens
SET consumed_at = $1
WHERE id = $2 AND consumed_at IS NULL;

-- name: CountActiveTokensByUser :one
SELECT COUNT(*) FROM verification_tokens
WHERE user_id = $1
  AND purpose = $2
  AND consumed_at IS NULL
  AND expires_at > $3;

-- name: RevokeUnconsumedTokensByUser :exec
UPDATE verification_tokens
SET consumed_at = $1
WHERE user_id = $2
  AND purpose = $3
  AND consumed_at IS NULL;

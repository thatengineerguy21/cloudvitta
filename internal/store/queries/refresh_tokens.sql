-- name: InsertRefreshToken :one
INSERT INTO refresh_tokens (
    user_id,
    family_id,
    token_hash,
    expires_at
) VALUES (
    $1, $2, $3, $4
)
RETURNING id, user_id, family_id, token_hash, created_at, expires_at, revoked_at, replaced_by;

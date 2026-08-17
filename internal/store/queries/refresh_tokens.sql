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

-- name: GetRefreshTokenByHashForUpdate :one
SELECT id, user_id, family_id, token_hash, created_at, expires_at, revoked_at, replaced_by
FROM refresh_tokens
WHERE token_hash = $1
FOR UPDATE;

-- name: GetRefreshTokenByID :one
SELECT id, user_id, family_id, token_hash, created_at, expires_at, revoked_at, replaced_by
FROM refresh_tokens
WHERE id = $1;

-- name: RevokeRefreshTokenWithReplacement :exec
UPDATE refresh_tokens
SET revoked_at = $2,
    replaced_by = $3
WHERE id = $1;

-- name: RevokeRefreshTokenByHash :exec
UPDATE refresh_tokens
SET revoked_at = $2
WHERE token_hash = $1 AND revoked_at IS NULL;

-- name: RevokeRefreshTokenFamily :exec
UPDATE refresh_tokens
SET revoked_at = $2
WHERE family_id = $1 AND revoked_at IS NULL;

-- name: ListRefreshTokensByFamilyID :many
SELECT id, user_id, family_id, token_hash, created_at, expires_at, revoked_at, replaced_by
FROM refresh_tokens
WHERE family_id = $1
ORDER BY created_at ASC;

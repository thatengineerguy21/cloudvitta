-- name: CreateUser :one
INSERT INTO users (
    email,
    password_hash,
    email_verified
) VALUES (
    $1, $2, $3
)
RETURNING id, email, password_hash, created_at, email_verified;

-- name: GetUserByEmail :one
SELECT id, email, password_hash, created_at, email_verified
FROM users
WHERE email = $1;

-- name: GetUserByID :one
-- Note: Retained for Stage 1.9 user profile/tier lookup during token refresh rotation.
SELECT id, email, password_hash, created_at, email_verified
FROM users
WHERE id = $1;

-- name: MarkEmailVerified :exec
UPDATE users
SET email_verified = true
WHERE id = $1;

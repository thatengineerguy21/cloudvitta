-- name: CreateUser :one
INSERT INTO users (
    email,
    password_hash
) VALUES (
    $1, $2
)
RETURNING id, email, password_hash, created_at;

-- name: GetUserByEmail :one
SELECT id, email, password_hash, created_at
FROM users
WHERE email = $1;

-- name: GetUserByID :one
-- Note: Retained for Stage 1.9 user profile/tier lookup during token refresh rotation.
SELECT id, email, password_hash, created_at
FROM users
WHERE id = $1;

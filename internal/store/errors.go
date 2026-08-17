package store

import (
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	// ErrNotFound is returned when a requested record is not found in the database.
	ErrNotFound = errors.New("store: record not found")

	// ErrDuplicateEmail is returned when attempting to insert a user with an email that already exists.
	ErrDuplicateEmail = errors.New("store: duplicate email")
)

// IsNotFound checks if an error represents a "record not found" condition.
func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, ErrNotFound) || errors.Is(err, pgx.ErrNoRows)
}

// IsUniqueViolation checks if an error represents a unique constraint violation (Postgres code 23505).
func IsUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrDuplicateEmail) {
		return true
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return true
	}
	errMsg := strings.ToLower(err.Error())
	return strings.Contains(errMsg, "unique") || strings.Contains(errMsg, "users_email_key")
}

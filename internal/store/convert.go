package store

import (
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// UUIDToPg converts a standard google uuid.UUID to a pgtype.UUID.
func UUIDToPg(u uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: u, Valid: true}
}

// PgToUUID converts a pgtype.UUID to a standard google uuid.UUID.
func PgToUUID(u pgtype.UUID) uuid.UUID {
	if u.Valid {
		return uuid.UUID(u.Bytes)
	}
	return uuid.Nil
}

// TimestamptzFromTime converts a time.Time to a pgtype.Timestamptz.
func TimestamptzFromTime(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

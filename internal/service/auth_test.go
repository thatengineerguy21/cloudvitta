package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/thatengineerguy21/CloudVitta/internal/auth"
	"github.com/thatengineerguy21/CloudVitta/internal/service"
	"github.com/thatengineerguy21/CloudVitta/internal/store"
)

var testJWTSecret = []byte("super-secret-jwt-key-with-at-least-32-bytes-length!")

// testRow implements pgx.Row for mocked query responses.
type testRow struct {
	scanFn func(dest ...interface{}) error
}

func (r *testRow) Scan(dest ...interface{}) error {
	if r.scanFn != nil {
		return r.scanFn(dest...)
	}
	return pgx.ErrNoRows
}

// testDBTX implements store.DBTX to verify Queries calls directly.
type testDBTX struct {
	queryRowFn func(ctx context.Context, sql string, args ...interface{}) pgx.Row
}

func (t *testDBTX) Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (t *testDBTX) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	return nil, nil
}

func (t *testDBTX) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	if t.queryRowFn != nil {
		return t.queryRowFn(ctx, sql, args...)
	}
	return &testRow{}
}

func TestSignup_Success(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()
	email := "newuser@example.com"
	password := "securePassword123"

	dbtx := &testDBTX{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) pgx.Row {
			if len(args) < 2 {
				t.Fatalf("expected at least 2 args, got %d", len(args))
			}
			argEmail, _ := args[0].(string)
			argHash, _ := args[1].(string)
			if argEmail != email {
				t.Errorf("argEmail = %q, want %q", argEmail, email)
			}
			if err := auth.CheckPassword(password, argHash); err != nil {
				t.Errorf("stored password hash does not match password: %v", err)
			}
			return &testRow{
				scanFn: func(dest ...interface{}) error {
					if idPtr, ok := dest[0].(*pgtype.UUID); ok {
						*idPtr = store.UUIDToPg(userUUID)
					}
					if emailPtr, ok := dest[1].(*string); ok {
						*emailPtr = argEmail
					}
					if hashPtr, ok := dest[2].(*string); ok {
						*hashPtr = argHash
					}
					if createdPtr, ok := dest[3].(*pgtype.Timestamptz); ok {
						*createdPtr = store.TimestamptzFromTime(time.Now().UTC())
					}
					return nil
				},
			}
		},
	}

	queries := store.New(dbtx)
	authSvc := service.NewAuthService(queries, testJWTSecret)
	user, err := authSvc.Signup(ctx, email, password)
	if err != nil {
		t.Fatalf("Signup failed: %v", err)
	}

	if user.ID != userUUID {
		t.Errorf("user.ID = %v, want %v", user.ID, userUUID)
	}
	if user.Email != email {
		t.Errorf("user.Email = %q, want %q", user.Email, email)
	}
}

func TestSignup_CaseInsensitiveEmail(t *testing.T) {
	ctx := context.Background()
	emailInput := "  User.Name@Example.COM  "
	expectedEmail := "user.name@example.com"
	userUUID := uuid.New()

	dbtx := &testDBTX{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) pgx.Row {
			argEmail, _ := args[0].(string)
			if argEmail != expectedEmail {
				t.Errorf("CreateUser called with %q, want normalized %q", argEmail, expectedEmail)
			}
			return &testRow{
				scanFn: func(dest ...interface{}) error {
					if idPtr, ok := dest[0].(*pgtype.UUID); ok {
						*idPtr = store.UUIDToPg(userUUID)
					}
					if emailPtr, ok := dest[1].(*string); ok {
						*emailPtr = argEmail
					}
					if hashPtr, ok := dest[2].(*string); ok {
						*hashPtr = "hashedPassword"
					}
					if createdPtr, ok := dest[3].(*pgtype.Timestamptz); ok {
						*createdPtr = store.TimestamptzFromTime(time.Now().UTC())
					}
					return nil
				},
			}
		},
	}

	queries := store.New(dbtx)
	authSvc := service.NewAuthService(queries, testJWTSecret)
	user, err := authSvc.Signup(ctx, emailInput, "securePassword123")
	if err != nil {
		t.Fatalf("Signup failed: %v", err)
	}
	if user.Email != expectedEmail {
		t.Errorf("returned user.Email = %q, want %q", user.Email, expectedEmail)
	}
}

func TestSignup_DuplicateEmail(t *testing.T) {
	ctx := context.Background()
	dbtx := &testDBTX{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) pgx.Row {
			return &testRow{
				scanFn: func(dest ...interface{}) error {
					return &pgconn.PgError{
						Code:           "23505",
						ConstraintName: "users_email_key",
					}
				},
			}
		},
	}

	queries := store.New(dbtx)
	authSvc := service.NewAuthService(queries, testJWTSecret)
	_, err := authSvc.Signup(ctx, "existing@example.com", "securePassword123")
	if !errors.Is(err, service.ErrUserAlreadyExists) {
		t.Errorf("expected ErrUserAlreadyExists, got: %v", err)
	}
}

func TestSignup_ValidationErrors(t *testing.T) {
	ctx := context.Background()
	dbtx := &testDBTX{}
	queries := store.New(dbtx)
	authSvc := service.NewAuthService(queries, testJWTSecret)

	// Invalid email
	_, err := authSvc.Signup(ctx, "notanemail", "securePassword123")
	if !errors.Is(err, service.ErrInvalidEmail) {
		t.Errorf("expected ErrInvalidEmail, got: %v", err)
	}

	// Short password
	_, err = authSvc.Signup(ctx, "valid@example.com", "short")
	if !errors.Is(err, service.ErrPasswordTooShort) {
		t.Errorf("expected ErrPasswordTooShort, got: %v", err)
	}
}

func TestLogin_Success(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()
	email := "loginuser@example.com"
	password := "correctPassword123"

	passwordHash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword failed: %v", err)
	}

	var insertedTokenHash string
	dbtx := &testDBTX{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) pgx.Row {
			if strings.Contains(sql, "FROM users") {
				return &testRow{
					scanFn: func(dest ...interface{}) error {
						if idPtr, ok := dest[0].(*pgtype.UUID); ok {
							*idPtr = store.UUIDToPg(userUUID)
						}
						if emailPtr, ok := dest[1].(*string); ok {
							*emailPtr = email
						}
						if hashPtr, ok := dest[2].(*string); ok {
							*hashPtr = passwordHash
						}
						if createdPtr, ok := dest[3].(*pgtype.Timestamptz); ok {
							*createdPtr = store.TimestamptzFromTime(time.Now().UTC())
						}
						return nil
					},
				}
			}
			if strings.Contains(sql, "INSERT INTO refresh_tokens") {
				if len(args) >= 3 {
					if th, ok := args[2].(string); ok {
						insertedTokenHash = th
					}
				}
				return &testRow{
					scanFn: func(dest ...interface{}) error {
						if idPtr, ok := dest[0].(*pgtype.UUID); ok {
							*idPtr = store.UUIDToPg(uuid.New())
						}
						return nil
					},
				}
			}
			return &testRow{}
		},
	}

	fixedNow := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	queries := store.New(dbtx)
	authSvc := service.NewAuthService(queries, testJWTSecret, service.WithClock(func() time.Time { return fixedNow }))

	tokens, err := authSvc.Login(ctx, email, password)
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	if tokens.TokenType != "Bearer" {
		t.Errorf("TokenType = %q, want Bearer", tokens.TokenType)
	}
	if tokens.ExpiresIn != 900 {
		t.Errorf("ExpiresIn = %d, want 900", tokens.ExpiresIn)
	}
	if tokens.AccessToken == "" || tokens.RefreshToken == "" {
		t.Fatalf("expected non-empty access and refresh tokens")
	}

	// Validate access token claims
	claims, err := auth.ValidateAccessToken(tokens.AccessToken, testJWTSecret, fixedNow)
	if err != nil {
		t.Fatalf("ValidateAccessToken failed: %v", err)
	}
	if claims.Subject != userUUID.String() {
		t.Errorf("claims.Subject = %q, want %q", claims.Subject, userUUID.String())
	}
	if claims.Tier != "standard" {
		t.Errorf("claims.Tier = %q, want standard", claims.Tier)
	}

	// Verify refresh token hash was saved to DB
	expectedHash := auth.HashRefreshToken(tokens.RefreshToken)
	if insertedTokenHash != expectedHash {
		t.Errorf("insertedTokenHash = %q, want %q", insertedTokenHash, expectedHash)
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	ctx := context.Background()
	passwordHash, _ := auth.HashPassword("realPassword123")

	dbtx := &testDBTX{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) pgx.Row {
			return &testRow{
				scanFn: func(dest ...interface{}) error {
					if idPtr, ok := dest[0].(*pgtype.UUID); ok {
						*idPtr = store.UUIDToPg(uuid.New())
					}
					if emailPtr, ok := dest[1].(*string); ok {
						*emailPtr = "user@example.com"
					}
					if hashPtr, ok := dest[2].(*string); ok {
						*hashPtr = passwordHash
					}
					return nil
				},
			}
		},
	}

	queries := store.New(dbtx)
	authSvc := service.NewAuthService(queries, testJWTSecret)
	_, err := authSvc.Login(ctx, "user@example.com", "wrongPassword")
	if !errors.Is(err, service.ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials for wrong password, got: %v", err)
	}
}

func TestLogin_UserNotFound(t *testing.T) {
	ctx := context.Background()
	dbtx := &testDBTX{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) pgx.Row {
			return &testRow{
				scanFn: func(dest ...interface{}) error {
					return pgx.ErrNoRows
				},
			}
		},
	}

	queries := store.New(dbtx)
	authSvc := service.NewAuthService(queries, testJWTSecret)
	_, err := authSvc.Login(ctx, "nonexistent@example.com", "somePassword123")
	if !errors.Is(err, service.ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials for nonexistent user, got: %v", err)
	}
}

func TestLogin_DatabaseError(t *testing.T) {
	ctx := context.Background()
	dbErr := errors.New("connection reset by peer")
	dbtx := &testDBTX{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) pgx.Row {
			return &testRow{
				scanFn: func(dest ...interface{}) error {
					return dbErr
				},
			}
		},
	}

	queries := store.New(dbtx)
	authSvc := service.NewAuthService(queries, testJWTSecret)
	_, err := authSvc.Login(ctx, "user@example.com", "somePassword123")
	// Database failure must NOT be masked as ErrInvalidCredentials
	if errors.Is(err, service.ErrInvalidCredentials) {
		t.Errorf("database error was erroneously masked as ErrInvalidCredentials")
	}
	if err == nil {
		t.Errorf("expected database error, got nil")
	}
}

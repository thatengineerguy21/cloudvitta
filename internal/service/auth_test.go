package service_test

import (
	"context"
	"errors"
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

// mockAuthStore implements store.Querier for testing AuthService in isolation.
type mockAuthStore struct {
	createUserFunc         func(ctx context.Context, arg store.CreateUserParams) (store.User, error)
	getUserByEmailFunc     func(ctx context.Context, email string) (store.User, error)
	insertRefreshTokenFunc func(ctx context.Context, arg store.InsertRefreshTokenParams) (store.RefreshToken, error)
}

func (m *mockAuthStore) CreateUser(ctx context.Context, arg store.CreateUserParams) (store.User, error) {
	if m.createUserFunc != nil {
		return m.createUserFunc(ctx, arg)
	}
	return store.User{}, errors.New("CreateUser not implemented")
}

func (m *mockAuthStore) GetUserByEmail(ctx context.Context, email string) (store.User, error) {
	if m.getUserByEmailFunc != nil {
		return m.getUserByEmailFunc(ctx, email)
	}
	return store.User{}, pgx.ErrNoRows
}

func (m *mockAuthStore) InsertRefreshToken(ctx context.Context, arg store.InsertRefreshTokenParams) (store.RefreshToken, error) {
	if m.insertRefreshTokenFunc != nil {
		return m.insertRefreshTokenFunc(ctx, arg)
	}
	return store.RefreshToken{
		ID:        pgtype.UUID{Bytes: uuid.New(), Valid: true},
		UserID:    arg.UserID,
		FamilyID:  arg.FamilyID,
		TokenHash: arg.TokenHash,
		ExpiresAt: arg.ExpiresAt,
	}, nil
}

func (m *mockAuthStore) GetUserByID(ctx context.Context, id pgtype.UUID) (store.User, error) {
	return store.User{}, errors.New("GetUserByID not implemented")
}

func (m *mockAuthStore) GetPriceObservations(ctx context.Context, arg store.GetPriceObservationsParams) ([]store.PriceObservation, error) {
	return nil, errors.New("GetPriceObservations not implemented")
}

func (m *mockAuthStore) GetLatestPriceForSKU(ctx context.Context, arg store.GetLatestPriceForSKUParams) (store.PriceObservation, error) {
	return store.PriceObservation{}, errors.New("GetLatestPriceForSKU not implemented")
}

func (m *mockAuthStore) InsertPriceObservation(ctx context.Context, arg store.InsertPriceObservationParams) (int64, error) {
	return 0, errors.New("InsertPriceObservation not implemented")
}

func TestSignup_Success(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()
	email := "newuser@example.com"
	password := "securePassword123"

	mock := &mockAuthStore{
		createUserFunc: func(ctx context.Context, arg store.CreateUserParams) (store.User, error) {
			if arg.Email != email {
				t.Errorf("arg.Email = %q, want %q", arg.Email, email)
			}
			if err := auth.CheckPassword(password, arg.PasswordHash); err != nil {
				t.Errorf("stored password hash does not match password: %v", err)
			}
			return store.User{
				ID:           pgtype.UUID{Bytes: userUUID, Valid: true},
				Email:        arg.Email,
				PasswordHash: arg.PasswordHash,
				CreatedAt:    pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
			}, nil
		},
	}

	authSvc := service.NewAuthService(mock, testJWTSecret)
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

	mock := &mockAuthStore{
		createUserFunc: func(ctx context.Context, arg store.CreateUserParams) (store.User, error) {
			if arg.Email != expectedEmail {
				t.Errorf("CreateUser called with %q, want normalized %q", arg.Email, expectedEmail)
			}
			return store.User{
				ID:           pgtype.UUID{Bytes: uuid.New(), Valid: true},
				Email:        arg.Email,
				PasswordHash: arg.PasswordHash,
				CreatedAt:    pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
			}, nil
		},
	}

	authSvc := service.NewAuthService(mock, testJWTSecret)
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
	mock := &mockAuthStore{
		createUserFunc: func(ctx context.Context, arg store.CreateUserParams) (store.User, error) {
			return store.User{}, &pgconn.PgError{
				Code:           "23505",
				ConstraintName: "users_email_key",
			}
		},
	}

	authSvc := service.NewAuthService(mock, testJWTSecret)
	_, err := authSvc.Signup(ctx, "existing@example.com", "securePassword123")
	if !errors.Is(err, service.ErrUserAlreadyExists) {
		t.Errorf("expected ErrUserAlreadyExists, got: %v", err)
	}
}

func TestSignup_ValidationErrors(t *testing.T) {
	ctx := context.Background()
	mock := &mockAuthStore{}
	authSvc := service.NewAuthService(mock, testJWTSecret)

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
	mock := &mockAuthStore{
		getUserByEmailFunc: func(ctx context.Context, e string) (store.User, error) {
			if e != email {
				return store.User{}, pgx.ErrNoRows
			}
			return store.User{
				ID:           pgtype.UUID{Bytes: userUUID, Valid: true},
				Email:        email,
				PasswordHash: passwordHash,
				CreatedAt:    pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
			}, nil
		},
		insertRefreshTokenFunc: func(ctx context.Context, arg store.InsertRefreshTokenParams) (store.RefreshToken, error) {
			insertedTokenHash = arg.TokenHash
			return store.RefreshToken{
				ID:        pgtype.UUID{Bytes: uuid.New(), Valid: true},
				UserID:    arg.UserID,
				FamilyID:  arg.FamilyID,
				TokenHash: arg.TokenHash,
				ExpiresAt: arg.ExpiresAt,
			}, nil
		},
	}

	fixedNow := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	authSvc := service.NewAuthService(mock, testJWTSecret, service.WithClock(func() time.Time { return fixedNow }))

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

	mock := &mockAuthStore{
		getUserByEmailFunc: func(ctx context.Context, email string) (store.User, error) {
			return store.User{
				ID:           pgtype.UUID{Bytes: uuid.New(), Valid: true},
				Email:        email,
				PasswordHash: passwordHash,
			}, nil
		},
	}

	authSvc := service.NewAuthService(mock, testJWTSecret)
	_, err := authSvc.Login(ctx, "user@example.com", "wrongPassword")
	if !errors.Is(err, service.ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials for wrong password, got: %v", err)
	}
}

func TestLogin_UserNotFound(t *testing.T) {
	ctx := context.Background()
	mock := &mockAuthStore{
		getUserByEmailFunc: func(ctx context.Context, email string) (store.User, error) {
			return store.User{}, pgx.ErrNoRows
		},
	}

	authSvc := service.NewAuthService(mock, testJWTSecret)
	_, err := authSvc.Login(ctx, "nonexistent@example.com", "somePassword123")
	if !errors.Is(err, service.ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials for nonexistent user, got: %v", err)
	}
}

func TestLogin_DatabaseError(t *testing.T) {
	ctx := context.Background()
	dbErr := errors.New("connection reset by peer")
	mock := &mockAuthStore{
		getUserByEmailFunc: func(ctx context.Context, email string) (store.User, error) {
			return store.User{}, dbErr
		},
	}

	authSvc := service.NewAuthService(mock, testJWTSecret)
	_, err := authSvc.Login(ctx, "user@example.com", "somePassword123")
	// Database failure must NOT be masked as ErrInvalidCredentials
	if errors.Is(err, service.ErrInvalidCredentials) {
		t.Errorf("database error was erroneously masked as ErrInvalidCredentials")
	}
	if err == nil {
		t.Errorf("expected database error, got nil")
	}
}

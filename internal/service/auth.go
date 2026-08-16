package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/thatengineerguy21/CloudVitta/internal/auth"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/store"
)

// AuthOption defines functional configuration options for AuthService.
type AuthOption func(*AuthService)

// WithClock overrides the default time provider for deterministic testing.
func WithClock(clock func() time.Time) AuthOption {
	return func(s *AuthService) {
		s.clock = clock
	}
}

// AuthService orchestrates user registration, authentication, and token management.
type AuthService struct {
	queries   store.Querier
	jwtSecret []byte
	clock     func() time.Time
}

// NewAuthService creates a new AuthService instance.
func NewAuthService(queries store.Querier, jwtSecret []byte, opts ...AuthOption) *AuthService {
	s := &AuthService{
		queries:   queries,
		jwtSecret: jwtSecret,
		clock:     time.Now,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Signup registers a new user with a normalized email and bcrypt-hashed password.
func (s *AuthService) Signup(ctx context.Context, email, password string) (*domain.User, error) {
	normalizedEmail, err := auth.NormalizeAndValidateEmail(email)
	if err != nil {
		return nil, ErrInvalidEmail
	}

	if err := auth.ValidatePassword(password); err != nil {
		if errors.Is(err, auth.ErrPasswordTooShort) {
			return nil, ErrPasswordTooShort
		}
		if errors.Is(err, auth.ErrPasswordTooLong) {
			return nil, ErrPasswordTooLong
		}
		return nil, ErrInvalidParameters
	}

	passwordHash, err := auth.HashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	user, err := s.queries.CreateUser(ctx, store.CreateUserParams{
		Email:        normalizedEmail,
		PasswordHash: passwordHash,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, ErrUserAlreadyExists
		}
		if strings.Contains(strings.ToLower(err.Error()), "unique") || strings.Contains(strings.ToLower(err.Error()), "users_email_key") {
			return nil, ErrUserAlreadyExists
		}
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	var userID uuid.UUID
	if user.ID.Valid {
		userID = uuid.UUID(user.ID.Bytes)
	}

	return &domain.User{
		ID:           userID,
		Email:        user.Email,
		PasswordHash: user.PasswordHash,
		CreatedAt:    user.CreatedAt.Time,
	}, nil
}

// Login authenticates a user using timing-safe bcrypt comparison and issues an access token and refresh token pair.
func (s *AuthService) Login(ctx context.Context, email, password string) (*domain.TokenPair, error) {
	normalizedEmail, err := auth.NormalizeAndValidateEmail(email)
	if err != nil {
		_ = auth.CheckPasswordTimingSafe(false, "", password)
		return nil, ErrInvalidCredentials
	}

	user, err := s.queries.GetUserByEmail(ctx, normalizedEmail)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			_ = auth.CheckPasswordTimingSafe(false, "", password)
			return nil, ErrInvalidCredentials
		}
		// Return database failure as internal error rather than masking as invalid credentials
		return nil, fmt.Errorf("database query failed: %w", err)
	}

	if err := auth.CheckPasswordTimingSafe(true, user.PasswordHash, password); err != nil {
		return nil, ErrInvalidCredentials
	}

	now := s.clock().UTC()
	familyID := uuid.New()
	rawRefreshToken, tokenHash, err := auth.GenerateRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate refresh token: %w", err)
	}

	expiresAt := now.Add(30 * 24 * time.Hour)
	_, err = s.queries.InsertRefreshToken(ctx, store.InsertRefreshTokenParams{
		UserID:    user.ID,
		FamilyID:  pgtype.UUID{Bytes: familyID, Valid: true},
		TokenHash: tokenHash,
		ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to record refresh token: %w", err)
	}

	var userUUID uuid.UUID
	if user.ID.Valid {
		userUUID = uuid.UUID(user.ID.Bytes)
	}

	accessToken, err := auth.GenerateAccessToken(userUUID, "standard", s.jwtSecret, now, 15*time.Minute)
	if err != nil {
		return nil, fmt.Errorf("failed to generate access token: %w", err)
	}

	return &domain.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: rawRefreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    900,
	}, nil
}

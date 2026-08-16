package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
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
	queries   *store.Queries
	jwtSecret []byte
	clock     func() time.Time
}

// NewAuthService creates a new AuthService instance.
func NewAuthService(queries *store.Queries, jwtSecret []byte, opts ...AuthOption) *AuthService {
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

	if s.queries == nil {
		return nil, fmt.Errorf("auth service: database queries unavailable")
	}

	user, err := s.queries.CreateUser(ctx, store.CreateUserParams{
		Email:        normalizedEmail,
		PasswordHash: passwordHash,
	})
	if err != nil {
		if store.IsUniqueViolation(err) {
			return nil, ErrUserAlreadyExists
		}
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return &domain.User{
		ID:           store.PgToUUID(user.ID),
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

	if s.queries == nil {
		_ = auth.CheckPasswordTimingSafe(false, "", password)
		return nil, fmt.Errorf("auth service: database queries unavailable")
	}

	user, err := s.queries.GetUserByEmail(ctx, normalizedEmail)
	if err != nil {
		if store.IsNotFound(err) {
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
		FamilyID:  store.UUIDToPg(familyID),
		TokenHash: tokenHash,
		ExpiresAt: store.TimestamptzFromTime(expiresAt),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to record refresh token: %w", err)
	}

	userUUID := store.PgToUUID(user.ID)

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

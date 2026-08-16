package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"

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

// WithTransactor configures the database transaction runner for atomic operations.
func WithTransactor(transactor store.Transactor) AuthOption {
	return func(s *AuthService) {
		s.transactor = transactor
	}
}

// WithCacheTTL overrides the default rotation replay cache TTL.
func WithCacheTTL(ttl time.Duration) AuthOption {
	return func(s *AuthService) {
		s.cacheTTL = ttl
	}
}

// WithRotationCache injects a specific RotationCache instance.
func WithRotationCache(cache *auth.RotationCache) AuthOption {
	return func(s *AuthService) {
		s.rotationCache = cache
	}
}

// WithAuthTracer configures OpenTelemetry tracing for AuthService.
func WithAuthTracer(tracer trace.Tracer) AuthOption {
	return func(s *AuthService) {
		s.tracer = tracer
	}
}

// AuthService orchestrates user registration, authentication, token rotation, and revocation.
type AuthService struct {
	queries       store.Querier
	transactor    store.Transactor
	jwtSecret     []byte
	clock         func() time.Time
	cacheTTL      time.Duration
	rotationCache *auth.RotationCache
	tracer        trace.Tracer
}

// NewAuthService creates a new AuthService instance.
func NewAuthService(queries store.Querier, jwtSecret []byte, opts ...AuthOption) *AuthService {
	s := &AuthService{
		queries:       queries,
		jwtSecret:     jwtSecret,
		clock:         time.Now,
		cacheTTL:      10 * time.Second,
		rotationCache: auth.NewRotationCache(),
		tracer:        noop.NewTracerProvider().Tracer("auth-service"),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Signup registers a new user with a normalized email and bcrypt-hashed password.
func (s *AuthService) Signup(ctx context.Context, email, password string) (*domain.User, error) {
	ctx, span := s.tracer.Start(ctx, "AuthService.Signup")
	defer span.End()

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
	ctx, span := s.tracer.Start(ctx, "AuthService.Login")
	defer span.End()

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

// Refresh performs atomic refresh token rotation with idempotency-key gated replay and whole-family theft detection.
func (s *AuthService) Refresh(ctx context.Context, rawRefreshToken, idempotencyKey string) (*domain.TokenPair, error) {
	ctx, span := s.tracer.Start(ctx, "AuthService.Refresh")
	defer span.End()

	if strings.TrimSpace(rawRefreshToken) == "" {
		return nil, ErrInvalidToken
	}
	if strings.TrimSpace(idempotencyKey) == "" {
		return nil, ErrMissingIdempotencyKey
	}

	tokenHash := auth.HashRefreshToken(rawRefreshToken)
	now := s.clock().UTC()

	var resultPair *domain.TokenPair

	execFn := func(q store.Querier) error {
		row, err := q.GetRefreshTokenByHashForUpdate(ctx, tokenHash)
		if err != nil {
			if store.IsNotFound(err) {
				return ErrInvalidToken
			}
			return fmt.Errorf("failed to query refresh token: %w", err)
		}

		if now.After(row.ExpiresAt.Time) {
			return ErrExpiredToken
		}

		// Check revocation status
		if !row.RevokedAt.Valid || row.RevokedAt.Time.IsZero() {
			// Branch 1: Token is valid and unrevoked -> rotate token
			newRawToken, newTokenHash, err := auth.GenerateRefreshToken()
			if err != nil {
				return fmt.Errorf("failed to generate new refresh token: %w", err)
			}

			userUUID := store.PgToUUID(row.UserID)
			accessToken, err := auth.GenerateAccessToken(userUUID, "standard", s.jwtSecret, now, 15*time.Minute)
			if err != nil {
				return fmt.Errorf("failed to generate access token: %w", err)
			}

			newRow, err := q.InsertRefreshToken(ctx, store.InsertRefreshTokenParams{
				UserID:    row.UserID,
				FamilyID:  row.FamilyID,
				TokenHash: newTokenHash,
				ExpiresAt: store.TimestamptzFromTime(now.Add(30 * 24 * time.Hour)),
			})
			if err != nil {
				return fmt.Errorf("failed to insert replacement refresh token: %w", err)
			}

			err = q.RevokeRefreshTokenWithReplacement(ctx, store.RevokeRefreshTokenWithReplacementParams{
				ID:         row.ID,
				RevokedAt:  store.TimestamptzFromTime(now),
				ReplacedBy: newRow.ID,
			})
			if err != nil {
				return fmt.Errorf("failed to revoke old refresh token: %w", err)
			}

			resultPair = &domain.TokenPair{
				AccessToken:  accessToken,
				RefreshToken: newRawToken,
				TokenType:    "Bearer",
				ExpiresIn:    900,
			}

			s.rotationCache.Put(tokenHash, idempotencyKey, *resultPair, now, s.cacheTTL)
			return nil
		}

		// Branch 2: Token was already revoked -> check replay cache for benign retry vs theft
		cached, ok := s.rotationCache.Get(tokenHash, now)
		if ok && cached != nil && cached.IdempotencyKey == idempotencyKey {
			// Case 2A: Benign replay within cache TTL with identical idempotency key
			resultPair = &cached.TokenPair
			return nil
		}

		// Case 2B/2C: Theft detected (mismatched idempotency key or cache miss/expired)
		// Immediately revoke the entire token family in PostgreSQL
		familyUUID := store.PgToUUID(row.FamilyID)
		tokenUUID := store.PgToUUID(row.ID)
		userUUID := store.PgToUUID(row.UserID)

		revokeErr := q.RevokeRefreshTokenFamily(ctx, store.RevokeRefreshTokenFamilyParams{
			FamilyID:  row.FamilyID,
			RevokedAt: store.TimestamptzFromTime(now),
		})
		if revokeErr != nil {
			slog.ErrorContext(ctx, "failed to revoke compromised token family",
				"family_id", familyUUID.String(),
				"user_id", userUUID.String(),
				"error", revokeErr,
			)
		}

		reason := "cache_miss_token_reused"
		if ok && cached != nil {
			reason = "mismatched_idempotency_key"
		}

		slog.WarnContext(ctx, "auth token theft detected: revoked entire token family",
			"family_id", familyUUID.String(),
			"token_id", tokenUUID.String(),
			"user_id", userUUID.String(),
			"reason", reason,
		)

		return ErrTokenFamilyRevoked
	}

	if s.transactor != nil {
		if err := s.transactor.ExecTx(ctx, execFn); err != nil {
			return nil, err
		}
	} else if s.queries != nil {
		if err := execFn(s.queries); err != nil {
			return nil, err
		}
	} else {
		return nil, fmt.Errorf("auth service: database unavailable for refresh")
	}

	return resultPair, nil
}

// Logout revokes the specified refresh token row and purges it from the replay cache.
func (s *AuthService) Logout(ctx context.Context, rawRefreshToken string) error {
	ctx, span := s.tracer.Start(ctx, "AuthService.Logout")
	defer span.End()

	if strings.TrimSpace(rawRefreshToken) == "" {
		return ErrInvalidToken
	}

	tokenHash := auth.HashRefreshToken(rawRefreshToken)
	now := s.clock().UTC()

	if s.queries != nil {
		_ = s.queries.RevokeRefreshTokenByHash(ctx, store.RevokeRefreshTokenByHashParams{
			TokenHash: tokenHash,
			RevokedAt: store.TimestamptzFromTime(now),
		})
	}

	s.rotationCache.Delete(tokenHash)
	return nil
}

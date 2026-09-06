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
	"github.com/thatengineerguy21/CloudVitta/internal/email"
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

// WithEmailSender configures the transactional email sender for AuthService.
func WithEmailSender(sender email.Sender) AuthOption {
	return func(s *AuthService) {
		s.emailSender = sender
	}
}

// WithVerifyBaseURL configures the base URL for email verification links.
func WithVerifyBaseURL(baseURL string) AuthOption {
	return func(s *AuthService) {
		s.verifyBaseURL = baseURL
	}
}

const (
	verificationTokenTTL   = 30 * time.Minute
	maxActiveVerifications = 3
	verificationPurpose    = "email_verification"
)

// AuthService orchestrates user registration, authentication, token rotation, and revocation.
type AuthService struct {
	queries       store.Querier
	transactor    store.Transactor
	jwtSecret     []byte
	clock         func() time.Time
	cacheTTL      time.Duration
	rotationCache *auth.RotationCache
	tracer        trace.Tracer
	emailSender   email.Sender
	verifyBaseURL string
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
		emailSender:   &email.NoopSender{},
		verifyBaseURL: "https://cloudvitta.dev/verify-email",
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
		Email:         normalizedEmail,
		PasswordHash:  passwordHash,
		EmailVerified: false,
	})
	if err != nil {
		if store.IsUniqueViolation(err) {
			return nil, ErrUserAlreadyExists
		}
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	rawToken, tokenHash, err := auth.GenerateRefreshToken()
	if err != nil {
		return nil, fmt.Errorf("failed to generate verification token: %w", err)
	}

	now := s.clock().UTC()
	_, err = s.queries.InsertVerificationToken(ctx, store.InsertVerificationTokenParams{
		UserID:    user.ID,
		TokenHash: tokenHash,
		Purpose:   verificationPurpose,
		ExpiresAt: store.TimestamptzFromTime(now.Add(verificationTokenTTL)),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to record verification token: %w", err)
	}

	if s.emailSender != nil {
		if err := s.sendVerificationEmail(ctx, normalizedEmail, rawToken); err != nil {
			slog.ErrorContext(ctx, "failed to send verification email",
				"user_id", store.PgToUUID(user.ID), "error", err)
		}
	}

	return &domain.User{
		ID:            store.PgToUUID(user.ID),
		Email:         user.Email,
		PasswordHash:  user.PasswordHash,
		EmailVerified: user.EmailVerified,
		CreatedAt:     user.CreatedAt.Time,
	}, nil
}

func (s *AuthService) sendVerificationEmail(ctx context.Context, toEmail, rawToken string) error {
	baseURL := s.verifyBaseURL
	if baseURL == "" {
		baseURL = "https://cloudvitta.dev/verify-email"
	}
	verifyURL := fmt.Sprintf("%s?token=%s", baseURL, rawToken)
	msg := email.Message{
		To:      toEmail,
		Subject: "Verify your CloudVitta email",
		HTML:    fmt.Sprintf(`<p>Click <a href="%s">here</a> to verify your email. This link expires in 30 minutes.</p>`, verifyURL),
		Text:    fmt.Sprintf("Verify your email: %s\nThis link expires in 30 minutes.", verifyURL),
	}
	return s.emailSender.Send(ctx, msg)
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

	if !user.EmailVerified {
		return nil, ErrEmailNotVerified
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
		if err := s.queries.RevokeRefreshTokenByHash(ctx, store.RevokeRefreshTokenByHashParams{
			TokenHash: tokenHash,
			RevokedAt: store.TimestamptzFromTime(now),
		}); err != nil {
			return fmt.Errorf("failed to revoke refresh token: %w", err)
		}
	}

	s.rotationCache.Delete(tokenHash)
	return nil
}

// VerifyEmail consumes a single-use verification token and activates the corresponding user account.
func (s *AuthService) VerifyEmail(ctx context.Context, rawToken string) error {
	ctx, span := s.tracer.Start(ctx, "AuthService.VerifyEmail")
	defer span.End()

	trimmed := strings.TrimSpace(rawToken)
	if trimmed == "" {
		return ErrVerificationNotFound
	}

	tokenHash := auth.HashRefreshToken(trimmed)

	vt, err := s.queries.GetVerificationTokenByHash(ctx, tokenHash)
	if err != nil {
		if store.IsNotFound(err) {
			return ErrVerificationNotFound
		}
		return fmt.Errorf("lookup verification token: %w", err)
	}

	now := s.clock().UTC()

	// Already consumed?
	if vt.ConsumedAt.Valid {
		return ErrVerificationConsumed
	}

	// Expired?
	if now.After(vt.ExpiresAt.Time) {
		return ErrVerificationExpired
	}

	// Atomically consume token + mark user verified in a transaction.
	nowPg := store.TimestamptzFromTime(now)
	if s.transactor != nil {
		return s.transactor.ExecTx(ctx, func(q store.Querier) error {
			if err := q.ConsumeVerificationToken(ctx, store.ConsumeVerificationTokenParams{
				ConsumedAt: nowPg,
				ID:         vt.ID,
			}); err != nil {
				return fmt.Errorf("consume verification token: %w", err)
			}
			if err := q.MarkEmailVerified(ctx, vt.UserID); err != nil {
				return fmt.Errorf("mark user email verified: %w", err)
			}
			return nil
		})
	}

	if err := s.queries.ConsumeVerificationToken(ctx, store.ConsumeVerificationTokenParams{
		ConsumedAt: nowPg,
		ID:         vt.ID,
	}); err != nil {
		return fmt.Errorf("consume verification token: %w", err)
	}
	if err := s.queries.MarkEmailVerified(ctx, vt.UserID); err != nil {
		return fmt.Errorf("mark user email verified: %w", err)
	}
	return nil
}

// ResendVerification issues a fresh verification token and emails the user if eligible.
// Returns nil for nonexistent or already verified users to prevent account enumeration.
func (s *AuthService) ResendVerification(ctx context.Context, emailAddr string) error {
	ctx, span := s.tracer.Start(ctx, "AuthService.ResendVerification")
	defer span.End()

	normalizedEmail, err := auth.NormalizeAndValidateEmail(emailAddr)
	if err != nil {
		return nil // silent - uniform 202 to avoid email enumeration
	}

	if s.queries == nil {
		return fmt.Errorf("auth service: database queries unavailable")
	}

	user, err := s.queries.GetUserByEmail(ctx, normalizedEmail)
	if err != nil {
		if store.IsNotFound(err) {
			return nil // silent - uniform 202 to avoid email enumeration
		}
		return fmt.Errorf("database query failed: %w", err)
	}

	if user.EmailVerified {
		return nil // already verified, uniform 202
	}

	// Rate limit: max N active tokens per user
	now := s.clock().UTC()
	nowPg := store.TimestamptzFromTime(now)
	count, err := s.queries.CountActiveTokensByUser(ctx, store.CountActiveTokensByUserParams{
		UserID:    user.ID,
		Purpose:   verificationPurpose,
		ExpiresAt: nowPg,
	})
	if err != nil {
		return fmt.Errorf("count active tokens: %w", err)
	}
	if count >= maxActiveVerifications {
		return ErrTooManyVerifications
	}

	rawToken, tokenHash, err := auth.GenerateRefreshToken()
	if err != nil {
		return fmt.Errorf("generate verification token: %w", err)
	}

	_, err = s.queries.InsertVerificationToken(ctx, store.InsertVerificationTokenParams{
		UserID:    user.ID,
		TokenHash: tokenHash,
		Purpose:   verificationPurpose,
		ExpiresAt: store.TimestamptzFromTime(now.Add(verificationTokenTTL)),
	})
	if err != nil {
		return fmt.Errorf("store verification token: %w", err)
	}

	if s.emailSender != nil {
		if err := s.sendVerificationEmail(ctx, normalizedEmail, rawToken); err != nil {
			slog.ErrorContext(ctx, "failed to send verification email",
				"user_id", store.PgToUUID(user.ID), "error", err)
		}
	}

	return nil
}

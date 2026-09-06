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
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/email"
	"github.com/thatengineerguy21/CloudVitta/internal/service"
	"github.com/thatengineerguy21/CloudVitta/internal/store"
)

var testJWTSecret = []byte("super-secret-jwt-key-with-at-least-32-bytes-length!")

// mockQuerier implements store.Querier for testing AuthService in isolation.
type mockQuerier struct {
	store.Querier
	createUserFunc                        func(ctx context.Context, arg store.CreateUserParams) (store.User, error)
	getUserByEmailFunc                    func(ctx context.Context, email string) (store.User, error)
	getUserByIDFunc                       func(ctx context.Context, id pgtype.UUID) (store.User, error)
	insertRefreshTokenFunc                func(ctx context.Context, arg store.InsertRefreshTokenParams) (store.RefreshToken, error)
	getRefreshTokenByHashForUpdateFunc    func(ctx context.Context, tokenHash string) (store.RefreshToken, error)
	getRefreshTokenByIDFunc               func(ctx context.Context, id pgtype.UUID) (store.RefreshToken, error)
	revokeRefreshTokenWithReplacementFunc func(ctx context.Context, arg store.RevokeRefreshTokenWithReplacementParams) error
	revokeRefreshTokenByHashFunc          func(ctx context.Context, arg store.RevokeRefreshTokenByHashParams) error
	revokeRefreshTokenFamilyFunc          func(ctx context.Context, arg store.RevokeRefreshTokenFamilyParams) error
	listRefreshTokensByFamilyIDFunc       func(ctx context.Context, familyID pgtype.UUID) ([]store.RefreshToken, error)
	insertVerificationTokenFunc           func(ctx context.Context, arg store.InsertVerificationTokenParams) (store.VerificationToken, error)
	getVerificationTokenByHashFunc        func(ctx context.Context, tokenHash string) (store.VerificationToken, error)
	consumeVerificationTokenFunc          func(ctx context.Context, arg store.ConsumeVerificationTokenParams) error
	countActiveTokensByUserFunc           func(ctx context.Context, arg store.CountActiveTokensByUserParams) (int64, error)
	markEmailVerifiedFunc                 func(ctx context.Context, id pgtype.UUID) error
	revokeUnconsumedTokensByUserFunc      func(ctx context.Context, arg store.RevokeUnconsumedTokensByUserParams) error
}

func (m *mockQuerier) CreateUser(ctx context.Context, arg store.CreateUserParams) (store.User, error) {
	if m.createUserFunc != nil {
		return m.createUserFunc(ctx, arg)
	}
	return store.User{}, errors.New("CreateUser not implemented")
}

func (m *mockQuerier) GetUserByEmail(ctx context.Context, email string) (store.User, error) {
	if m.getUserByEmailFunc != nil {
		return m.getUserByEmailFunc(ctx, email)
	}
	return store.User{}, pgx.ErrNoRows
}

func (m *mockQuerier) GetUserByID(ctx context.Context, id pgtype.UUID) (store.User, error) {
	if m.getUserByIDFunc != nil {
		return m.getUserByIDFunc(ctx, id)
	}
	return store.User{}, pgx.ErrNoRows
}

func (m *mockQuerier) InsertRefreshToken(ctx context.Context, arg store.InsertRefreshTokenParams) (store.RefreshToken, error) {
	if m.insertRefreshTokenFunc != nil {
		return m.insertRefreshTokenFunc(ctx, arg)
	}
	return store.RefreshToken{
		ID:        store.UUIDToPg(uuid.New()),
		UserID:    arg.UserID,
		FamilyID:  arg.FamilyID,
		TokenHash: arg.TokenHash,
		ExpiresAt: arg.ExpiresAt,
	}, nil
}

func (m *mockQuerier) GetRefreshTokenByHashForUpdate(ctx context.Context, tokenHash string) (store.RefreshToken, error) {
	if m.getRefreshTokenByHashForUpdateFunc != nil {
		return m.getRefreshTokenByHashForUpdateFunc(ctx, tokenHash)
	}
	return store.RefreshToken{}, pgx.ErrNoRows
}

func (m *mockQuerier) GetRefreshTokenByID(ctx context.Context, id pgtype.UUID) (store.RefreshToken, error) {
	if m.getRefreshTokenByIDFunc != nil {
		return m.getRefreshTokenByIDFunc(ctx, id)
	}
	return store.RefreshToken{}, pgx.ErrNoRows
}

func (m *mockQuerier) RevokeRefreshTokenWithReplacement(ctx context.Context, arg store.RevokeRefreshTokenWithReplacementParams) error {
	if m.revokeRefreshTokenWithReplacementFunc != nil {
		return m.revokeRefreshTokenWithReplacementFunc(ctx, arg)
	}
	return nil
}

func (m *mockQuerier) RevokeRefreshTokenByHash(ctx context.Context, arg store.RevokeRefreshTokenByHashParams) error {
	if m.revokeRefreshTokenByHashFunc != nil {
		return m.revokeRefreshTokenByHashFunc(ctx, arg)
	}
	return nil
}

func (m *mockQuerier) RevokeRefreshTokenFamily(ctx context.Context, arg store.RevokeRefreshTokenFamilyParams) error {
	if m.revokeRefreshTokenFamilyFunc != nil {
		return m.revokeRefreshTokenFamilyFunc(ctx, arg)
	}
	return nil
}

func (m *mockQuerier) ListRefreshTokensByFamilyID(ctx context.Context, familyID pgtype.UUID) ([]store.RefreshToken, error) {
	if m.listRefreshTokensByFamilyIDFunc != nil {
		return m.listRefreshTokensByFamilyIDFunc(ctx, familyID)
	}
	return nil, nil
}

func (m *mockQuerier) InsertVerificationToken(ctx context.Context, arg store.InsertVerificationTokenParams) (store.VerificationToken, error) {
	if m.insertVerificationTokenFunc != nil {
		return m.insertVerificationTokenFunc(ctx, arg)
	}
	return store.VerificationToken{
		ID:        store.UUIDToPg(uuid.New()),
		UserID:    arg.UserID,
		TokenHash: arg.TokenHash,
		Purpose:   arg.Purpose,
		ExpiresAt: arg.ExpiresAt,
	}, nil
}

func (m *mockQuerier) GetVerificationTokenByHash(ctx context.Context, tokenHash string) (store.VerificationToken, error) {
	if m.getVerificationTokenByHashFunc != nil {
		return m.getVerificationTokenByHashFunc(ctx, tokenHash)
	}
	return store.VerificationToken{}, pgx.ErrNoRows
}

func (m *mockQuerier) ConsumeVerificationToken(ctx context.Context, arg store.ConsumeVerificationTokenParams) error {
	if m.consumeVerificationTokenFunc != nil {
		return m.consumeVerificationTokenFunc(ctx, arg)
	}
	return nil
}

func (m *mockQuerier) CountActiveTokensByUser(ctx context.Context, arg store.CountActiveTokensByUserParams) (int64, error) {
	if m.countActiveTokensByUserFunc != nil {
		return m.countActiveTokensByUserFunc(ctx, arg)
	}
	return 0, nil
}

func (m *mockQuerier) MarkEmailVerified(ctx context.Context, id pgtype.UUID) error {
	if m.markEmailVerifiedFunc != nil {
		return m.markEmailVerifiedFunc(ctx, id)
	}
	return nil
}

func (m *mockQuerier) RevokeUnconsumedTokensByUser(ctx context.Context, arg store.RevokeUnconsumedTokensByUserParams) error {
	if m.revokeUnconsumedTokensByUserFunc != nil {
		return m.revokeUnconsumedTokensByUserFunc(ctx, arg)
	}
	return nil
}

func (m *mockQuerier) GetPriceObservations(ctx context.Context, arg store.GetPriceObservationsParams) ([]store.PriceObservation, error) {
	return nil, errors.New("GetPriceObservations not implemented")
}

func (m *mockQuerier) GetLatestPriceForSKU(ctx context.Context, arg store.GetLatestPriceForSKUParams) (store.PriceObservation, error) {
	return store.PriceObservation{}, errors.New("GetLatestPriceForSKU not implemented")
}

func (m *mockQuerier) GetLatestPriceForSKUAndCategory(ctx context.Context, arg store.GetLatestPriceForSKUAndCategoryParams) (store.PriceObservation, error) {
	return store.PriceObservation{}, errors.New("GetLatestPriceForSKUAndCategory not implemented")
}

func (m *mockQuerier) UpdatePriceObservationLastSeenAt(ctx context.Context, arg store.UpdatePriceObservationLastSeenAtParams) error {
	return nil
}

func (m *mockQuerier) InsertPriceObservation(ctx context.Context, arg store.InsertPriceObservationParams) (int64, error) {
	return 0, errors.New("InsertPriceObservation not implemented")
}

func (m *mockQuerier) GetProviderCategoryStatus(ctx context.Context, provider string) ([]store.GetProviderCategoryStatusRow, error) {
	return nil, errors.New("GetProviderCategoryStatus not implemented")
}

type mockTransactor struct {
	q store.Querier
}

func (m *mockTransactor) ExecTx(ctx context.Context, fn func(q store.Querier) error) error {
	return fn(m.q)
}

type mockEmailSender struct {
	sendFunc func(ctx context.Context, msg email.Message) error
	sent     []email.Message
}

func (m *mockEmailSender) Send(ctx context.Context, msg email.Message) error {
	m.sent = append(m.sent, msg)
	if m.sendFunc != nil {
		return m.sendFunc(ctx, msg)
	}
	return nil
}

func TestSignup_Success(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()
	email := "newuser@example.com"
	password := "securePassword123"

	mock := &mockQuerier{
		createUserFunc: func(ctx context.Context, arg store.CreateUserParams) (store.User, error) {
			if arg.Email != email {
				t.Errorf("arg.Email = %q, want %q", arg.Email, email)
			}
			if err := auth.CheckPassword(password, arg.PasswordHash); err != nil {
				t.Errorf("stored password hash does not match password: %v", err)
			}
			return store.User{
				ID:           store.UUIDToPg(userUUID),
				Email:        arg.Email,
				PasswordHash: arg.PasswordHash,
				CreatedAt:    store.TimestamptzFromTime(time.Now().UTC()),
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

	mock := &mockQuerier{
		createUserFunc: func(ctx context.Context, arg store.CreateUserParams) (store.User, error) {
			if arg.Email != expectedEmail {
				t.Errorf("CreateUser called with %q, want normalized %q", arg.Email, expectedEmail)
			}
			return store.User{
				ID:           store.UUIDToPg(uuid.New()),
				Email:        arg.Email,
				PasswordHash: arg.PasswordHash,
				CreatedAt:    store.TimestamptzFromTime(time.Now().UTC()),
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
	mock := &mockQuerier{
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
	mock := &mockQuerier{}
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

	// Long password (> 72 chars)
	_, err = authSvc.Signup(ctx, "valid@example.com", strings.Repeat("a", 73))
	if !errors.Is(err, service.ErrPasswordTooLong) {
		t.Errorf("expected ErrPasswordTooLong, got: %v", err)
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
	mock := &mockQuerier{
		getUserByEmailFunc: func(ctx context.Context, e string) (store.User, error) {
			if e != email {
				return store.User{}, pgx.ErrNoRows
			}
			return store.User{
				ID:            store.UUIDToPg(userUUID),
				Email:         email,
				PasswordHash:  passwordHash,
				EmailVerified: true,
				CreatedAt:     store.TimestamptzFromTime(time.Now().UTC()),
			}, nil
		},
		insertRefreshTokenFunc: func(ctx context.Context, arg store.InsertRefreshTokenParams) (store.RefreshToken, error) {
			insertedTokenHash = arg.TokenHash
			return store.RefreshToken{
				ID:        store.UUIDToPg(uuid.New()),
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

	mock := &mockQuerier{
		getUserByEmailFunc: func(ctx context.Context, email string) (store.User, error) {
			return store.User{
				ID:           store.UUIDToPg(uuid.New()),
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
	mock := &mockQuerier{
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
	mock := &mockQuerier{
		getUserByEmailFunc: func(ctx context.Context, email string) (store.User, error) {
			return store.User{}, dbErr
		},
	}

	authSvc := service.NewAuthService(mock, testJWTSecret)
	_, err := authSvc.Login(ctx, "user@example.com", "somePassword123")
	if errors.Is(err, service.ErrInvalidCredentials) {
		t.Errorf("database error was erroneously masked as ErrInvalidCredentials")
	}
	if err == nil {
		t.Errorf("expected database error, got nil")
	}
}

func TestRefresh_ValidationErrors(t *testing.T) {
	ctx := context.Background()
	mock := &mockQuerier{}
	authSvc := service.NewAuthService(mock, testJWTSecret)

	// Missing refresh token
	_, err := authSvc.Refresh(ctx, "", "idemp-key-1")
	if !errors.Is(err, service.ErrInvalidToken) {
		t.Errorf("expected ErrInvalidToken for empty refresh token, got: %v", err)
	}

	// Missing idempotency key
	_, err = authSvc.Refresh(ctx, "valid-raw-refresh-token", "")
	if !errors.Is(err, service.ErrMissingIdempotencyKey) {
		t.Errorf("expected ErrMissingIdempotencyKey for empty key, got: %v", err)
	}

	// Whitespace idempotency key
	_, err = authSvc.Refresh(ctx, "valid-raw-refresh-token", "   ")
	if !errors.Is(err, service.ErrMissingIdempotencyKey) {
		t.Errorf("expected ErrMissingIdempotencyKey for whitespace key, got: %v", err)
	}
}

func TestRefresh_ExpiredToken(t *testing.T) {
	ctx := context.Background()
	rawToken := "some-raw-token"
	tokenHash := auth.HashRefreshToken(rawToken)
	now := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)

	mock := &mockQuerier{
		getRefreshTokenByHashForUpdateFunc: func(ctx context.Context, hash string) (store.RefreshToken, error) {
			if hash != tokenHash {
				return store.RefreshToken{}, pgx.ErrNoRows
			}
			return store.RefreshToken{
				ID:        store.UUIDToPg(uuid.New()),
				UserID:    store.UUIDToPg(uuid.New()),
				FamilyID:  store.UUIDToPg(uuid.New()),
				TokenHash: tokenHash,
				ExpiresAt: store.TimestamptzFromTime(now.Add(-1 * time.Hour)), // Expired 1 hour ago
			}, nil
		},
	}

	authSvc := service.NewAuthService(mock, testJWTSecret,
		service.WithClock(func() time.Time { return now }),
		service.WithTransactor(&mockTransactor{q: mock}),
	)

	_, err := authSvc.Refresh(ctx, rawToken, "idemp-key-1")
	if !errors.Is(err, service.ErrExpiredToken) {
		t.Errorf("expected ErrExpiredToken, got: %v", err)
	}
}

func TestRefresh_Success_Unrevoked(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()
	familyUUID := uuid.New()
	oldTokenID := uuid.New()

	rawOldToken := "old-raw-token-123456"
	oldTokenHash := auth.HashRefreshToken(rawOldToken)
	now := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)

	var insertedNewToken store.RefreshToken
	var revokedOldTokenID pgtype.UUID
	var replacementID pgtype.UUID

	mock := &mockQuerier{
		getRefreshTokenByHashForUpdateFunc: func(ctx context.Context, hash string) (store.RefreshToken, error) {
			if hash != oldTokenHash {
				return store.RefreshToken{}, pgx.ErrNoRows
			}
			return store.RefreshToken{
				ID:        store.UUIDToPg(oldTokenID),
				UserID:    store.UUIDToPg(userUUID),
				FamilyID:  store.UUIDToPg(familyUUID),
				TokenHash: oldTokenHash,
				ExpiresAt: store.TimestamptzFromTime(now.Add(30 * 24 * time.Hour)),
				RevokedAt: pgtype.Timestamptz{}, // Unrevoked
			}, nil
		},
		insertRefreshTokenFunc: func(ctx context.Context, arg store.InsertRefreshTokenParams) (store.RefreshToken, error) {
			insertedNewToken = store.RefreshToken{
				ID:        store.UUIDToPg(uuid.New()),
				UserID:    arg.UserID,
				FamilyID:  arg.FamilyID,
				TokenHash: arg.TokenHash,
				ExpiresAt: arg.ExpiresAt,
			}
			return insertedNewToken, nil
		},
		revokeRefreshTokenWithReplacementFunc: func(ctx context.Context, arg store.RevokeRefreshTokenWithReplacementParams) error {
			revokedOldTokenID = arg.ID
			replacementID = arg.ReplacedBy
			return nil
		},
	}

	authSvc := service.NewAuthService(mock, testJWTSecret,
		service.WithClock(func() time.Time { return now }),
		service.WithTransactor(&mockTransactor{q: mock}),
	)

	idempotencyKey := "client-idemp-key-1"
	pair, err := authSvc.Refresh(ctx, rawOldToken, idempotencyKey)
	if err != nil {
		t.Fatalf("Refresh failed: %v", err)
	}

	if pair.AccessToken == "" || pair.RefreshToken == "" {
		t.Errorf("expected non-empty tokens in pair")
	}
	if pair.ExpiresIn != 900 {
		t.Errorf("ExpiresIn = %d, want 900", pair.ExpiresIn)
	}

	// Verify old token was revoked with replacement
	if store.PgToUUID(revokedOldTokenID) != oldTokenID {
		t.Errorf("revokedOldTokenID = %v, want %v", store.PgToUUID(revokedOldTokenID), oldTokenID)
	}
	if store.PgToUUID(replacementID) != store.PgToUUID(insertedNewToken.ID) {
		t.Errorf("replacementID = %v, want %v", store.PgToUUID(replacementID), store.PgToUUID(insertedNewToken.ID))
	}
}

func TestRefresh_BenignReplay_SameIdempotencyKey(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()
	familyUUID := uuid.New()
	oldTokenID := uuid.New()

	rawOldToken := "old-raw-token-replay-test"
	oldTokenHash := auth.HashRefreshToken(rawOldToken)
	now := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	idempotencyKey := "same-idemp-key"

	mock := &mockQuerier{
		getRefreshTokenByHashForUpdateFunc: func(ctx context.Context, hash string) (store.RefreshToken, error) {
			return store.RefreshToken{
				ID:        store.UUIDToPg(oldTokenID),
				UserID:    store.UUIDToPg(userUUID),
				FamilyID:  store.UUIDToPg(familyUUID),
				TokenHash: oldTokenHash,
				ExpiresAt: store.TimestamptzFromTime(now.Add(30 * 24 * time.Hour)),
				RevokedAt: store.TimestamptzFromTime(now.Add(-2 * time.Second)), // Already revoked 2s ago
			}, nil
		},
	}

	rotationCache := auth.NewRotationCache()
	cachedPair := domain.TokenPair{
		AccessToken:  "cached.access.jwt",
		RefreshToken: "cached-refresh-token",
		TokenType:    "Bearer",
		ExpiresIn:    900,
	}
	// Seed replay cache (as if first request just completed)
	rotationCache.Put(oldTokenHash, idempotencyKey, cachedPair, now.Add(-2*time.Second), 10*time.Second)

	authSvc := service.NewAuthService(mock, testJWTSecret,
		service.WithClock(func() time.Time { return now }),
		service.WithTransactor(&mockTransactor{q: mock}),
		service.WithRotationCache(rotationCache),
	)

	// Replay with identical idempotency key within 10s TTL
	pair, err := authSvc.Refresh(ctx, rawOldToken, idempotencyKey)
	if err != nil {
		t.Fatalf("expected benign replay success, got error: %v", err)
	}

	if pair.AccessToken != cachedPair.AccessToken || pair.RefreshToken != cachedPair.RefreshToken {
		t.Errorf("expected cached token pair returned on benign replay, got %+v", pair)
	}
}

func TestRefresh_TheftDetection_MismatchedKey(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()
	familyUUID := uuid.New()
	oldTokenID := uuid.New()

	rawOldToken := "compromised-raw-token"
	oldTokenHash := auth.HashRefreshToken(rawOldToken)
	now := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)

	var revokedFamilyID pgtype.UUID
	mock := &mockQuerier{
		getRefreshTokenByHashForUpdateFunc: func(ctx context.Context, hash string) (store.RefreshToken, error) {
			return store.RefreshToken{
				ID:        store.UUIDToPg(oldTokenID),
				UserID:    store.UUIDToPg(userUUID),
				FamilyID:  store.UUIDToPg(familyUUID),
				TokenHash: oldTokenHash,
				ExpiresAt: store.TimestamptzFromTime(now.Add(30 * 24 * time.Hour)),
				RevokedAt: store.TimestamptzFromTime(now.Add(-2 * time.Second)), // Revoked 2s ago
			}, nil
		},
		revokeRefreshTokenFamilyFunc: func(ctx context.Context, arg store.RevokeRefreshTokenFamilyParams) error {
			revokedFamilyID = arg.FamilyID
			return nil
		},
	}

	rotationCache := auth.NewRotationCache()
	rotationCache.Put(oldTokenHash, "legitimate-client-key", domain.TokenPair{AccessToken: "token"}, now.Add(-2*time.Second), 10*time.Second)

	authSvc := service.NewAuthService(mock, testJWTSecret,
		service.WithClock(func() time.Time { return now }),
		service.WithTransactor(&mockTransactor{q: mock}),
		service.WithRotationCache(rotationCache),
	)

	// Attacker presents token with DIFFERENT idempotency key
	_, err := authSvc.Refresh(ctx, rawOldToken, "attacker-mismatched-key")
	if !errors.Is(err, service.ErrTokenFamilyRevoked) {
		t.Fatalf("expected ErrTokenFamilyRevoked on mismatched idempotency key, got: %v", err)
	}

	if store.PgToUUID(revokedFamilyID) != familyUUID {
		t.Errorf("revokedFamilyID = %v, want %v", store.PgToUUID(revokedFamilyID), familyUUID)
	}
}

func TestRefresh_TheftDetection_CacheMissOrExpired(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()
	familyUUID := uuid.New()
	oldTokenID := uuid.New()

	rawOldToken := "reused-outside-cache-window-token"
	oldTokenHash := auth.HashRefreshToken(rawOldToken)
	now := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)

	var revokedFamilyID pgtype.UUID
	mock := &mockQuerier{
		getRefreshTokenByHashForUpdateFunc: func(ctx context.Context, hash string) (store.RefreshToken, error) {
			return store.RefreshToken{
				ID:        store.UUIDToPg(oldTokenID),
				UserID:    store.UUIDToPg(userUUID),
				FamilyID:  store.UUIDToPg(familyUUID),
				TokenHash: oldTokenHash,
				ExpiresAt: store.TimestamptzFromTime(now.Add(30 * 24 * time.Hour)),
				RevokedAt: store.TimestamptzFromTime(now.Add(-60 * time.Second)), // Revoked 60s ago (past 10s TTL)
			}, nil
		},
		revokeRefreshTokenFamilyFunc: func(ctx context.Context, arg store.RevokeRefreshTokenFamilyParams) error {
			revokedFamilyID = arg.FamilyID
			return nil
		},
	}

	rotationCache := auth.NewRotationCache() // Empty / expired cache

	authSvc := service.NewAuthService(mock, testJWTSecret,
		service.WithClock(func() time.Time { return now }),
		service.WithTransactor(&mockTransactor{q: mock}),
		service.WithRotationCache(rotationCache),
	)

	_, err := authSvc.Refresh(ctx, rawOldToken, "any-idemp-key")
	if !errors.Is(err, service.ErrTokenFamilyRevoked) {
		t.Fatalf("expected ErrTokenFamilyRevoked on cache miss, got: %v", err)
	}

	if store.PgToUUID(revokedFamilyID) != familyUUID {
		t.Errorf("revokedFamilyID = %v, want %v", store.PgToUUID(revokedFamilyID), familyUUID)
	}
}

func TestLogout_Success(t *testing.T) {
	ctx := context.Background()
	rawToken := "logout-raw-token-xyz"
	tokenHash := auth.HashRefreshToken(rawToken)

	var revokedHash string
	mock := &mockQuerier{
		revokeRefreshTokenByHashFunc: func(ctx context.Context, arg store.RevokeRefreshTokenByHashParams) error {
			revokedHash = arg.TokenHash
			return nil
		},
	}

	rotationCache := auth.NewRotationCache()
	rotationCache.Put(tokenHash, "key", domain.TokenPair{AccessToken: "jwt"}, time.Now().UTC(), 10*time.Second)

	authSvc := service.NewAuthService(mock, testJWTSecret, service.WithRotationCache(rotationCache))

	err := authSvc.Logout(ctx, rawToken)
	if err != nil {
		t.Fatalf("Logout failed: %v", err)
	}

	if revokedHash != tokenHash {
		t.Errorf("revokedHash = %q, want %q", revokedHash, tokenHash)
	}

	// Verify evicted from rotation cache
	if _, ok := rotationCache.Get(tokenHash, time.Now().UTC()); ok {
		t.Errorf("expected token evicted from rotation cache after logout")
	}

	// Empty token validation
	if err := authSvc.Logout(ctx, ""); !errors.Is(err, service.ErrInvalidToken) {
		t.Errorf("expected ErrInvalidToken for empty logout token, got: %v", err)
	}
}

func TestLogout_DatabaseError(t *testing.T) {
	ctx := context.Background()
	rawToken := "logout-raw-token-xyz"

	dbErr := errors.New("database connection failed")
	mock := &mockQuerier{
		revokeRefreshTokenByHashFunc: func(ctx context.Context, arg store.RevokeRefreshTokenByHashParams) error {
			return dbErr
		},
	}

	authSvc := service.NewAuthService(mock, testJWTSecret)
	err := authSvc.Logout(ctx, rawToken)
	if err == nil {
		t.Fatalf("expected error on DB failure during logout, got nil")
	}
}

func TestSignup_SendsVerificationEmail(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()
	userEmail := "verifytest@example.com"
	password := "securePassword123"

	var capturedUser store.CreateUserParams
	var capturedToken store.InsertVerificationTokenParams
	emailSender := &mockEmailSender{}

	mock := &mockQuerier{
		createUserFunc: func(ctx context.Context, arg store.CreateUserParams) (store.User, error) {
			capturedUser = arg
			return store.User{
				ID:            store.UUIDToPg(userUUID),
				Email:         arg.Email,
				PasswordHash:  arg.PasswordHash,
				EmailVerified: arg.EmailVerified,
				CreatedAt:     store.TimestamptzFromTime(time.Now().UTC()),
			}, nil
		},
		insertVerificationTokenFunc: func(ctx context.Context, arg store.InsertVerificationTokenParams) (store.VerificationToken, error) {
			capturedToken = arg
			return store.VerificationToken{
				ID:        store.UUIDToPg(uuid.New()),
				UserID:    arg.UserID,
				TokenHash: arg.TokenHash,
				Purpose:   arg.Purpose,
				ExpiresAt: arg.ExpiresAt,
			}, nil
		},
	}

	authSvc := service.NewAuthService(mock, testJWTSecret,
		service.WithEmailSender(emailSender),
		service.WithVerifyBaseURL("https://cloudvitta.dev/verify-email"),
	)

	user, err := authSvc.Signup(ctx, userEmail, password)
	if err != nil {
		t.Fatalf("Signup failed: %v", err)
	}

	if user.EmailVerified {
		t.Errorf("expected user.EmailVerified = false, got true")
	}
	if capturedUser.EmailVerified {
		t.Errorf("expected capturedUser.EmailVerified = false, got true")
	}
	if capturedToken.TokenHash == "" {
		t.Errorf("expected verification token hash to be populated")
	}
	if capturedToken.Purpose != "email_verification" {
		t.Errorf("expected purpose 'email_verification', got %q", capturedToken.Purpose)
	}
	if len(emailSender.sent) != 1 {
		t.Fatalf("expected 1 email sent, got %d", len(emailSender.sent))
	}
	sentMsg := emailSender.sent[0]
	if sentMsg.To != userEmail {
		t.Errorf("sentMsg.To = %q, want %q", sentMsg.To, userEmail)
	}
	if !strings.Contains(sentMsg.HTML, "https://cloudvitta.dev/verify-email?token=") {
		t.Errorf("expected verify URL in HTML, got %s", sentMsg.HTML)
	}
}

func TestLogin_RejectsUnverifiedUser(t *testing.T) {
	ctx := context.Background()
	userEmail := "unverified@example.com"
	password := "securePassword123"
	hash, _ := auth.HashPassword(password)

	mock := &mockQuerier{
		getUserByEmailFunc: func(ctx context.Context, email string) (store.User, error) {
			return store.User{
				ID:            store.UUIDToPg(uuid.New()),
				Email:         userEmail,
				PasswordHash:  hash,
				EmailVerified: false,
			}, nil
		},
	}

	authSvc := service.NewAuthService(mock, testJWTSecret)
	_, err := authSvc.Login(ctx, userEmail, password)
	if !errors.Is(err, service.ErrEmailNotVerified) {
		t.Fatalf("expected ErrEmailNotVerified, got: %v", err)
	}
}

func TestLogin_AllowsVerifiedUser(t *testing.T) {
	ctx := context.Background()
	userEmail := "verified@example.com"
	password := "securePassword123"
	hash, _ := auth.HashPassword(password)

	mock := &mockQuerier{
		getUserByEmailFunc: func(ctx context.Context, email string) (store.User, error) {
			return store.User{
				ID:            store.UUIDToPg(uuid.New()),
				Email:         userEmail,
				PasswordHash:  hash,
				EmailVerified: true,
			}, nil
		},
	}

	authSvc := service.NewAuthService(mock, testJWTSecret)
	tokens, err := authSvc.Login(ctx, userEmail, password)
	if err != nil {
		t.Fatalf("expected login to succeed, got: %v", err)
	}
	if tokens.AccessToken == "" || tokens.RefreshToken == "" {
		t.Errorf("expected access and refresh tokens, got %+v", tokens)
	}
}

func TestVerifyEmail_HappyPath(t *testing.T) {
	ctx := context.Background()
	rawToken := "raw-verification-token-123"
	tokenHash := auth.HashRefreshToken(rawToken)
	tokenID := uuid.New()
	userID := uuid.New()
	fixedNow := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)

	var consumedID uuid.UUID
	var verifiedUserID uuid.UUID

	mock := &mockQuerier{
		getVerificationTokenByHashFunc: func(ctx context.Context, hash string) (store.VerificationToken, error) {
			if hash != tokenHash {
				return store.VerificationToken{}, pgx.ErrNoRows
			}
			return store.VerificationToken{
				ID:        store.UUIDToPg(tokenID),
				UserID:    store.UUIDToPg(userID),
				TokenHash: hash,
				Purpose:   "email_verification",
				ExpiresAt: store.TimestamptzFromTime(fixedNow.Add(30 * time.Minute)),
			}, nil
		},
		consumeVerificationTokenFunc: func(ctx context.Context, arg store.ConsumeVerificationTokenParams) error {
			consumedID = store.PgToUUID(arg.ID)
			return nil
		},
		markEmailVerifiedFunc: func(ctx context.Context, id pgtype.UUID) error {
			verifiedUserID = store.PgToUUID(id)
			return nil
		},
	}

	authSvc := service.NewAuthService(mock, testJWTSecret,
		service.WithClock(func() time.Time { return fixedNow }),
		service.WithTransactor(&mockTransactor{q: mock}),
	)

	err := authSvc.VerifyEmail(ctx, rawToken)
	if err != nil {
		t.Fatalf("expected VerifyEmail to succeed, got: %v", err)
	}
	if consumedID != tokenID {
		t.Errorf("consumedID = %v, want %v", consumedID, tokenID)
	}
	if verifiedUserID != userID {
		t.Errorf("verifiedUserID = %v, want %v", verifiedUserID, userID)
	}
}

func TestVerifyEmail_ExpiredToken(t *testing.T) {
	ctx := context.Background()
	rawToken := "expired-token"
	tokenHash := auth.HashRefreshToken(rawToken)
	fixedNow := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)

	mock := &mockQuerier{
		getVerificationTokenByHashFunc: func(ctx context.Context, hash string) (store.VerificationToken, error) {
			if hash != tokenHash {
				return store.VerificationToken{}, pgx.ErrNoRows
			}
			return store.VerificationToken{
				ID:        store.UUIDToPg(uuid.New()),
				UserID:    store.UUIDToPg(uuid.New()),
				TokenHash: hash,
				Purpose:   "email_verification",
				ExpiresAt: store.TimestamptzFromTime(fixedNow.Add(-5 * time.Minute)), // expired 5 mins ago
			}, nil
		},
	}

	authSvc := service.NewAuthService(mock, testJWTSecret,
		service.WithClock(func() time.Time { return fixedNow }),
		service.WithTransactor(&mockTransactor{q: mock}),
	)

	err := authSvc.VerifyEmail(ctx, rawToken)
	if !errors.Is(err, service.ErrVerificationExpired) {
		t.Fatalf("expected ErrVerificationExpired, got: %v", err)
	}
}

func TestVerifyEmail_AlreadyConsumed(t *testing.T) {
	ctx := context.Background()
	rawToken := "consumed-token"
	tokenHash := auth.HashRefreshToken(rawToken)
	fixedNow := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)

	mock := &mockQuerier{
		getVerificationTokenByHashFunc: func(ctx context.Context, hash string) (store.VerificationToken, error) {
			if hash != tokenHash {
				return store.VerificationToken{}, pgx.ErrNoRows
			}
			return store.VerificationToken{
				ID:         store.UUIDToPg(uuid.New()),
				UserID:     store.UUIDToPg(uuid.New()),
				TokenHash:  hash,
				Purpose:    "email_verification",
				ExpiresAt:  store.TimestamptzFromTime(fixedNow.Add(30 * time.Minute)),
				ConsumedAt: store.TimestamptzFromTime(fixedNow.Add(-10 * time.Minute)),
			}, nil
		},
	}

	authSvc := service.NewAuthService(mock, testJWTSecret,
		service.WithClock(func() time.Time { return fixedNow }),
		service.WithTransactor(&mockTransactor{q: mock}),
	)

	err := authSvc.VerifyEmail(ctx, rawToken)
	if !errors.Is(err, service.ErrVerificationConsumed) {
		t.Fatalf("expected ErrVerificationConsumed, got: %v", err)
	}
}

func TestVerifyEmail_UnknownToken(t *testing.T) {
	ctx := context.Background()
	mock := &mockQuerier{
		getVerificationTokenByHashFunc: func(ctx context.Context, hash string) (store.VerificationToken, error) {
			return store.VerificationToken{}, pgx.ErrNoRows
		},
	}

	authSvc := service.NewAuthService(mock, testJWTSecret)
	err := authSvc.VerifyEmail(ctx, "nonexistent-token")
	if !errors.Is(err, service.ErrVerificationNotFound) {
		t.Fatalf("expected ErrVerificationNotFound, got: %v", err)
	}
}

func TestResendVerification_RateLimits(t *testing.T) {
	ctx := context.Background()
	userEmail := "ratelimit@example.com"
	mock := &mockQuerier{
		getUserByEmailFunc: func(ctx context.Context, email string) (store.User, error) {
			return store.User{
				ID:            store.UUIDToPg(uuid.New()),
				Email:         userEmail,
				EmailVerified: false,
			}, nil
		},
		countActiveTokensByUserFunc: func(ctx context.Context, arg store.CountActiveTokensByUserParams) (int64, error) {
			return 3, nil // max reached
		},
	}

	authSvc := service.NewAuthService(mock, testJWTSecret)
	err := authSvc.ResendVerification(ctx, userEmail)
	if !errors.Is(err, service.ErrTooManyVerifications) {
		t.Fatalf("expected ErrTooManyVerifications, got: %v", err)
	}
}

func TestResendVerification_NoEnumeration(t *testing.T) {
	ctx := context.Background()
	sender := &mockEmailSender{}

	// Case 1: Nonexistent user
	mockNonexistent := &mockQuerier{
		getUserByEmailFunc: func(ctx context.Context, email string) (store.User, error) {
			return store.User{}, pgx.ErrNoRows
		},
	}
	authSvc1 := service.NewAuthService(mockNonexistent, testJWTSecret, service.WithEmailSender(sender))
	if err := authSvc1.ResendVerification(ctx, "missing@example.com"); err != nil {
		t.Errorf("expected nil for nonexistent email, got %v", err)
	}
	if len(sender.sent) != 0 {
		t.Errorf("expected no emails sent for nonexistent user")
	}

	// Case 2: Already verified user
	mockVerified := &mockQuerier{
		getUserByEmailFunc: func(ctx context.Context, email string) (store.User, error) {
			return store.User{
				ID:            store.UUIDToPg(uuid.New()),
				Email:         email,
				EmailVerified: true,
			}, nil
		},
	}
	authSvc2 := service.NewAuthService(mockVerified, testJWTSecret, service.WithEmailSender(sender))
	if err := authSvc2.ResendVerification(ctx, "alreadyverified@example.com"); err != nil {
		t.Errorf("expected nil for already verified user, got %v", err)
	}
	if len(sender.sent) != 0 {
		t.Errorf("expected no emails sent for already verified user")
	}
}

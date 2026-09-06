package rest_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/thatengineerguy21/CloudVitta/internal/auth"
	"github.com/thatengineerguy21/CloudVitta/internal/service"
	"github.com/thatengineerguy21/CloudVitta/internal/store"
	"github.com/thatengineerguy21/CloudVitta/internal/transport/rest"
	"github.com/thatengineerguy21/CloudVitta/internal/transport/rest/middleware"
)

type mockTransactor struct {
	q store.Querier
}

func (m *mockTransactor) ExecTx(ctx context.Context, fn func(q store.Querier) error) error {
	return fn(m.q)
}

func TestVerifyEmailHandler_200(t *testing.T) {
	rawToken := "valid-token-123"
	tokenHash := auth.HashRefreshToken(rawToken)
	tokenID := uuid.New()
	userID := uuid.New()
	fixedNow := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)

	var consumed bool
	var verified bool

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
			consumed = true
			return nil
		},
		markEmailVerifiedFunc: func(ctx context.Context, id pgtype.UUID) error {
			verified = true
			return nil
		},
	}

	authSvc := service.NewAuthService(mock, testJWTSecret,
		service.WithClock(func() time.Time { return fixedNow }),
		service.WithTransactor(&mockTransactor{q: mock}),
	)
	handler := rest.NewVerifyEmailHandler(authSvc)

	body := `{"token": "valid-token-123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/verify-email", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 OK. Body: %s", rec.Code, rec.Body.String())
	}
	if !consumed || !verified {
		t.Errorf("expected token consumed and user verified, got consumed=%v, verified=%v", consumed, verified)
	}

	var resp rest.VerifyEmailResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Message != "Email verified successfully." {
		t.Errorf("unexpected message: %q", resp.Message)
	}
}

func TestVerifyEmailHandler_MissingToken(t *testing.T) {
	authSvc := service.NewAuthService(&mockQuerier{}, testJWTSecret)
	handler := rest.NewVerifyEmailHandler(authSvc)

	// Sub-test 1: Empty token string
	body := `{"token": ""}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/verify-email", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 Bad Request", rec.Code)
	}

	// Sub-test 2: Malformed JSON
	reqBad := httptest.NewRequest(http.MethodPost, "/api/v1/auth/verify-email", bytes.NewBufferString(`invalid-json`))
	reqBad.Header.Set("Content-Type", "application/json")
	recBad := httptest.NewRecorder()

	handler.ServeHTTP(recBad, reqBad)
	if recBad.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 Bad Request for malformed JSON", recBad.Code)
	}
}

func TestVerifyEmailHandler_ExpiredToken(t *testing.T) {
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
				ExpiresAt: store.TimestamptzFromTime(fixedNow.Add(-10 * time.Minute)),
			}, nil
		},
	}

	authSvc := service.NewAuthService(mock, testJWTSecret,
		service.WithClock(func() time.Time { return fixedNow }),
		service.WithTransactor(&mockTransactor{q: mock}),
	)
	handler := rest.NewVerifyEmailHandler(authSvc)

	body := `{"token": "expired-token"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/verify-email", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusGone {
		t.Fatalf("status = %d, want 410 Gone. Body: %s", rec.Code, rec.Body.String())
	}
}

func TestVerifyEmailHandler_ConsumedToken(t *testing.T) {
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
				ConsumedAt: store.TimestamptzFromTime(fixedNow.Add(-5 * time.Minute)),
			}, nil
		},
	}

	authSvc := service.NewAuthService(mock, testJWTSecret,
		service.WithClock(func() time.Time { return fixedNow }),
		service.WithTransactor(&mockTransactor{q: mock}),
	)
	handler := rest.NewVerifyEmailHandler(authSvc)

	body := `{"token": "consumed-token"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/verify-email", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 Conflict. Body: %s", rec.Code, rec.Body.String())
	}
}

func TestVerifyEmailHandler_NotFound(t *testing.T) {
	mock := &mockQuerier{
		getVerificationTokenByHashFunc: func(ctx context.Context, hash string) (store.VerificationToken, error) {
			return store.VerificationToken{}, pgx.ErrNoRows
		},
	}

	authSvc := service.NewAuthService(mock, testJWTSecret)
	handler := rest.NewVerifyEmailHandler(authSvc)

	body := `{"token": "missing-token"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/verify-email", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 Not Found. Body: %s", rec.Code, rec.Body.String())
	}
}

func TestResendVerificationHandler_202(t *testing.T) {
	userEmail := "unverified@example.com"
	mock := &mockQuerier{
		getUserByEmailFunc: func(ctx context.Context, email string) (store.User, error) {
			return store.User{
				ID:            store.UUIDToPg(uuid.New()),
				Email:         userEmail,
				EmailVerified: false,
			}, nil
		},
		countActiveTokensByUserFunc: func(ctx context.Context, arg store.CountActiveTokensByUserParams) (int64, error) {
			return 1, nil
		},
	}

	authSvc := service.NewAuthService(mock, testJWTSecret)
	handler := rest.NewResendVerificationHandler(authSvc)

	body := `{"email": "unverified@example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/resend-verification", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202 Accepted. Body: %s", rec.Code, rec.Body.String())
	}

	var resp rest.ResendVerificationResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Message == "" {
		t.Errorf("expected non-empty message")
	}
}

func TestResendVerificationHandler_MissingEmail(t *testing.T) {
	authSvc := service.NewAuthService(&mockQuerier{}, testJWTSecret)
	handler := rest.NewResendVerificationHandler(authSvc)

	body := `{"email": ""}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/resend-verification", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 Bad Request", rec.Code)
	}
}

func TestResendVerificationHandler_429(t *testing.T) {
	userEmail := "flooder@example.com"
	mock := &mockQuerier{
		getUserByEmailFunc: func(ctx context.Context, email string) (store.User, error) {
			return store.User{
				ID:            store.UUIDToPg(uuid.New()),
				Email:         userEmail,
				EmailVerified: false,
			}, nil
		},
		countActiveTokensByUserFunc: func(ctx context.Context, arg store.CountActiveTokensByUserParams) (int64, error) {
			return 3, nil // limit reached
		},
	}

	authSvc := service.NewAuthService(mock, testJWTSecret)
	handler := rest.NewResendVerificationHandler(authSvc)

	body := `{"email": "flooder@example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/resend-verification", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429 Too Many Requests. Body: %s", rec.Code, rec.Body.String())
	}
}

func TestLoginHandler_403_Unverified(t *testing.T) {
	userUUID := uuid.New()
	email := "unverified-user@example.com"
	password := "correctPassword123"
	passwordHash, _ := auth.HashPassword(password)

	mock := &mockQuerier{
		getUserByEmailFunc: func(ctx context.Context, e string) (store.User, error) {
			if e != email {
				return store.User{}, pgx.ErrNoRows
			}
			return store.User{
				ID:            store.UUIDToPg(userUUID),
				Email:         email,
				PasswordHash:  passwordHash,
				EmailVerified: false, // unverified!
				CreatedAt:     store.TimestamptzFromTime(time.Now().UTC()),
			}, nil
		},
	}

	authSvc := service.NewAuthService(mock, testJWTSecret)
	handler := rest.NewLoginHandler(authSvc)

	body := `{"email": "unverified-user@example.com", "password": "correctPassword123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403 Forbidden. Body: %s", rec.Code, rec.Body.String())
	}

	var rfcErr middleware.RFC7807Error
	if err := json.Unmarshal(rec.Body.Bytes(), &rfcErr); err != nil {
		t.Fatalf("failed to decode RFC 7807 error: %v", err)
	}
	if rfcErr.Type != "https://cloudvitta.dev/errors/email-not-verified" {
		t.Errorf("error type = %q, want email-not-verified URI", rfcErr.Type)
	}
}

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
	"github.com/thatengineerguy21/CloudVitta/internal/auth"
	"github.com/thatengineerguy21/CloudVitta/internal/service"
	"github.com/thatengineerguy21/CloudVitta/internal/store"
	"github.com/thatengineerguy21/CloudVitta/internal/transport/rest"
	"github.com/thatengineerguy21/CloudVitta/internal/transport/rest/middleware"
)

type restMockTransactor struct {
	q store.Querier
}

func (m *restMockTransactor) ExecTx(ctx context.Context, fn func(q store.Querier) error) error {
	return fn(m.q)
}

func TestRefreshHandler_Success(t *testing.T) {
	rawOldToken := "raw-refresh-token-123"
	oldHash := auth.HashRefreshToken(rawOldToken)
	userUUID := uuid.New()
	familyUUID := uuid.New()
	oldTokenID := uuid.New()
	now := time.Now().UTC()

	mock := &mockQuerier{
		getRefreshTokenByHashForUpdateFunc: func(ctx context.Context, hash string) (store.RefreshToken, error) {
			if hash != oldHash {
				return store.RefreshToken{}, pgx.ErrNoRows
			}
			return store.RefreshToken{
				ID:        store.UUIDToPg(oldTokenID),
				UserID:    store.UUIDToPg(userUUID),
				FamilyID:  store.UUIDToPg(familyUUID),
				TokenHash: oldHash,
				ExpiresAt: store.TimestamptzFromTime(now.Add(30 * 24 * time.Hour)),
				RevokedAt: store.TimestamptzFromTime(time.Time{}),
			}, nil
		},
		insertRefreshTokenFunc: func(ctx context.Context, arg store.InsertRefreshTokenParams) (store.RefreshToken, error) {
			return store.RefreshToken{
				ID:        store.UUIDToPg(uuid.New()),
				UserID:    arg.UserID,
				FamilyID:  arg.FamilyID,
				TokenHash: arg.TokenHash,
				ExpiresAt: arg.ExpiresAt,
			}, nil
		},
		revokeRefreshTokenWithReplacementFunc: func(ctx context.Context, arg store.RevokeRefreshTokenWithReplacementParams) error {
			return nil
		},
	}

	authSvc := service.NewAuthService(mock, testJWTSecret,
		service.WithTransactor(&restMockTransactor{q: mock}),
	)
	handler := rest.NewRefreshHandler(authSvc)

	body := `{"refresh_token": "raw-refresh-token-123", "idempotency_key": "idemp-key-1"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 OK. Body: %s", rec.Code, rec.Body.String())
	}

	var resp rest.RefreshResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.TokenType != "Bearer" {
		t.Errorf("TokenType = %q, want Bearer", resp.TokenType)
	}
	if resp.ExpiresIn != 900 {
		t.Errorf("ExpiresIn = %d, want 900", resp.ExpiresIn)
	}
	if resp.AccessToken == "" || resp.RefreshToken == "" {
		t.Errorf("expected non-empty rotated tokens")
	}
}

func TestRefreshHandler_ValidationErrors(t *testing.T) {
	authSvc := service.NewAuthService(&mockQuerier{}, testJWTSecret)
	handler := rest.NewRefreshHandler(authSvc)

	// 1. Missing idempotency key
	body := `{"refresh_token": "valid-token"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 Bad Request for missing idempotency key", rec.Code)
	}

	// 2. Missing refresh token
	body2 := `{"idempotency_key": "valid-key"}`
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewBufferString(body2))
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 Bad Request for missing refresh token", rec2.Code)
	}

	// 3. Malformed JSON
	req3 := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewBufferString(`{invalid`))
	rec3 := httptest.NewRecorder()
	handler.ServeHTTP(rec3, req3)

	if rec3.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 Bad Request for malformed JSON", rec3.Code)
	}
}

func TestRefreshHandler_TheftDetection_Returns401(t *testing.T) {
	rawOldToken := "stolen-refresh-token"
	oldHash := auth.HashRefreshToken(rawOldToken)
	now := time.Now().UTC()

	mock := &mockQuerier{
		getRefreshTokenByHashForUpdateFunc: func(ctx context.Context, hash string) (store.RefreshToken, error) {
			return store.RefreshToken{
				ID:        store.UUIDToPg(uuid.New()),
				UserID:    store.UUIDToPg(uuid.New()),
				FamilyID:  store.UUIDToPg(uuid.New()),
				TokenHash: oldHash,
				ExpiresAt: store.TimestamptzFromTime(now.Add(30 * 24 * time.Hour)),
				RevokedAt: store.TimestamptzFromTime(now.Add(-20 * time.Second)), // Revoked
			}, nil
		},
		revokeRefreshTokenFamilyFunc: func(ctx context.Context, arg store.RevokeRefreshTokenFamilyParams) error {
			return nil
		},
	}

	authSvc := service.NewAuthService(mock, testJWTSecret,
		service.WithTransactor(&restMockTransactor{q: mock}),
	)
	handler := rest.NewRefreshHandler(authSvc)

	body := `{"refresh_token": "stolen-refresh-token", "idempotency_key": "some-key"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 Unauthorized for theft detection. Body: %s", rec.Code, rec.Body.String())
	}

	var rfcErr middleware.RFC7807Error
	if err := json.Unmarshal(rec.Body.Bytes(), &rfcErr); err != nil {
		t.Fatalf("failed to decode RFC7807 error: %v", err)
	}
	if rfcErr.Type != "https://cloudvitta.dev/errors/unauthorized" {
		t.Errorf("error type = %q, want https://cloudvitta.dev/errors/unauthorized", rfcErr.Type)
	}
}

func TestRefreshHandler_MethodNotAllowed(t *testing.T) {
	authSvc := service.NewAuthService(&mockQuerier{}, testJWTSecret)
	handler := rest.NewRefreshHandler(authSvc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/refresh", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405 Method Not Allowed", rec.Code)
	}
}

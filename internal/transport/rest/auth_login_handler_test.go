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

func TestLoginHandler_Success(t *testing.T) {
	userUUID := uuid.New()
	email := "loginuser@example.com"
	password := "correctPassword123"
	passwordHash, _ := auth.HashPassword(password)

	mock := &mockQuerier{
		getUserByEmailFunc: func(ctx context.Context, e string) (store.User, error) {
			if e != email {
				return store.User{}, pgx.ErrNoRows
			}
			return store.User{
				ID:           store.UUIDToPg(userUUID),
				Email:        email,
				PasswordHash: passwordHash,
				CreatedAt:    store.TimestamptzFromTime(time.Now().UTC()),
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
	}

	authSvc := service.NewAuthService(mock, testJWTSecret)
	handler := rest.NewLoginHandler(authSvc)

	body := `{"email": "loginuser@example.com", "password": "correctPassword123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 OK. Body: %s", rec.Code, rec.Body.String())
	}

	var resp rest.LoginResponse
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
		t.Errorf("expected non-empty tokens in response")
	}
}

func TestLoginHandler_InvalidCredentials(t *testing.T) {
	mock := &mockQuerier{
		getUserByEmailFunc: func(ctx context.Context, e string) (store.User, error) {
			return store.User{}, pgx.ErrNoRows
		},
	}

	authSvc := service.NewAuthService(mock, testJWTSecret)
	handler := rest.NewLoginHandler(authSvc)

	body := `{"email": "wrong@example.com", "password": "wrongPassword123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401 Unauthorized. Body: %s", rec.Code, rec.Body.String())
	}

	var rfcErr middleware.RFC7807Error
	if err := json.Unmarshal(rec.Body.Bytes(), &rfcErr); err != nil {
		t.Fatalf("failed to decode RFC7807 error: %v", err)
	}
	if rfcErr.Type != "https://cloudvitta.dev/errors/unauthorized" {
		t.Errorf("error type = %q, want https://cloudvitta.dev/errors/unauthorized", rfcErr.Type)
	}
}

func TestLoginHandler_MethodNotAllowed(t *testing.T) {
	authSvc := service.NewAuthService(&mockQuerier{}, testJWTSecret)
	handler := rest.NewLoginHandler(authSvc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/login", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405 Method Not Allowed", rec.Code)
	}
}

func TestLoginHandler_MalformedJSON(t *testing.T) {
	authSvc := service.NewAuthService(&mockQuerier{}, testJWTSecret)
	handler := rest.NewLoginHandler(authSvc)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewBufferString(`{bad`))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 Bad Request", rec.Code)
	}
}

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
				ID:           pgtype.UUID{Bytes: userUUID, Valid: true},
				Email:        email,
				PasswordHash: passwordHash,
				CreatedAt:    pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true},
			}, nil
		},
		insertRefreshTokenFunc: func(ctx context.Context, arg store.InsertRefreshTokenParams) (store.RefreshToken, error) {
			return store.RefreshToken{
				ID:        pgtype.UUID{Bytes: uuid.New(), Valid: true},
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
		t.Errorf("expected non-empty access and refresh tokens")
	}
}

func TestLoginHandler_InvalidPassword(t *testing.T) {
	passwordHash, _ := auth.HashPassword("realPassword123")

	mock := &mockQuerier{
		getUserByEmailFunc: func(ctx context.Context, email string) (store.User, error) {
			return store.User{
				ID:           pgtype.UUID{Bytes: uuid.New(), Valid: true},
				Email:        email,
				PasswordHash: passwordHash,
			}, nil
		},
	}

	authSvc := service.NewAuthService(mock, testJWTSecret)
	handler := rest.NewLoginHandler(authSvc)

	body := `{"email": "user@example.com", "password": "wrongPassword123"}`
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

func TestLoginHandler_NonexistentEmail_IdenticalResponse(t *testing.T) {
	mock := &mockQuerier{
		getUserByEmailFunc: func(ctx context.Context, email string) (store.User, error) {
			return store.User{}, pgx.ErrNoRows
		},
	}

	authSvc := service.NewAuthService(mock, testJWTSecret)
	handler := rest.NewLoginHandler(authSvc)

	body := `{"email": "unknown@example.com", "password": "anyPassword123"}`
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
	if rfcErr.Title != "Unauthorized" {
		t.Errorf("error title = %q, want Unauthorized", rfcErr.Title)
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

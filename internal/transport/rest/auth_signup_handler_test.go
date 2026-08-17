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
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/thatengineerguy21/CloudVitta/internal/service"
	"github.com/thatengineerguy21/CloudVitta/internal/store"
	"github.com/thatengineerguy21/CloudVitta/internal/transport/rest"
	"github.com/thatengineerguy21/CloudVitta/internal/transport/rest/middleware"
)

var testJWTSecret = []byte("super-secret-jwt-key-with-at-least-32-bytes-length!")

func TestSignupHandler_Success(t *testing.T) {
	userUUID := uuid.New()
	mock := &mockQuerier{
		createUserFunc: func(ctx context.Context, arg store.CreateUserParams) (store.User, error) {
			return store.User{
				ID:           store.UUIDToPg(userUUID),
				Email:        arg.Email,
				PasswordHash: arg.PasswordHash,
				CreatedAt:    store.TimestamptzFromTime(time.Now().UTC()),
			}, nil
		},
	}

	authSvc := service.NewAuthService(mock, testJWTSecret)
	handler := rest.NewSignupHandler(authSvc)

	body := `{"email": "user@example.com", "password": "securePassword123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/signup", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 Created. Body: %s", rec.Code, rec.Body.String())
	}

	var resp rest.SignupResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.ID != userUUID {
		t.Errorf("resp.ID = %v, want %v", resp.ID, userUUID)
	}
	if resp.Email != "user@example.com" {
		t.Errorf("resp.Email = %q, want user@example.com", resp.Email)
	}
}

func TestSignupHandler_MethodNotAllowed(t *testing.T) {
	authSvc := service.NewAuthService(&mockQuerier{}, testJWTSecret)
	handler := rest.NewSignupHandler(authSvc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/signup", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405 Method Not Allowed", rec.Code)
	}
}

func TestSignupHandler_MalformedJSON(t *testing.T) {
	authSvc := service.NewAuthService(&mockQuerier{}, testJWTSecret)
	handler := rest.NewSignupHandler(authSvc)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/signup", bytes.NewBufferString(`{invalid-json`))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 Bad Request", rec.Code)
	}
}

func TestSignupHandler_ShortPassword(t *testing.T) {
	authSvc := service.NewAuthService(&mockQuerier{}, testJWTSecret)
	handler := rest.NewSignupHandler(authSvc)

	body := `{"email": "user@example.com", "password": "123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/signup", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 Bad Request", rec.Code)
	}

	var rfcErr middleware.RFC7807Error
	_ = json.Unmarshal(rec.Body.Bytes(), &rfcErr)
	if rfcErr.Type != "https://cloudvitta.dev/errors/invalid-parameter" {
		t.Errorf("error type = %q, want https://cloudvitta.dev/errors/invalid-parameter", rfcErr.Type)
	}
}

func TestSignupHandler_DuplicateEmail(t *testing.T) {
	mock := &mockQuerier{
		createUserFunc: func(ctx context.Context, arg store.CreateUserParams) (store.User, error) {
			return store.User{}, &pgconn.PgError{
				Code:           "23505",
				ConstraintName: "users_email_key",
			}
		},
	}

	authSvc := service.NewAuthService(mock, testJWTSecret)
	handler := rest.NewSignupHandler(authSvc)

	body := `{"email": "existing@example.com", "password": "securePassword123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/signup", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 Conflict. Body: %s", rec.Code, rec.Body.String())
	}

	var rfcErr middleware.RFC7807Error
	if err := json.Unmarshal(rec.Body.Bytes(), &rfcErr); err != nil {
		t.Fatalf("failed to decode RFC7807 error: %v", err)
	}
	if rfcErr.Type != "https://cloudvitta.dev/errors/conflict" {
		t.Errorf("error type = %q, want https://cloudvitta.dev/errors/conflict", rfcErr.Type)
	}
	if rfcErr.Title != "Conflict" {
		t.Errorf("error title = %q, want Conflict", rfcErr.Title)
	}
}

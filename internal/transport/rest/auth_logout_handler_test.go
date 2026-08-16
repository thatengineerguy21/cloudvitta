package rest_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/thatengineerguy21/CloudVitta/internal/service"
	"github.com/thatengineerguy21/CloudVitta/internal/store"
	"github.com/thatengineerguy21/CloudVitta/internal/transport/rest"
)

func TestLogoutHandler_Success(t *testing.T) {
	var revokedTokenHash string
	mock := &mockQuerier{
		revokeRefreshTokenByHashFunc: func(ctx context.Context, arg store.RevokeRefreshTokenByHashParams) error {
			revokedTokenHash = arg.TokenHash
			return nil
		},
	}

	authSvc := service.NewAuthService(mock, testJWTSecret)
	handler := rest.NewLogoutHandler(authSvc)

	body := `{"refresh_token": "valid-raw-refresh-token"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 OK. Body: %s", rec.Code, rec.Body.String())
	}

	var resp rest.LogoutResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Message != "logged out successfully" {
		t.Errorf("resp.Message = %q, want 'logged out successfully'", resp.Message)
	}

	if revokedTokenHash == "" {
		t.Errorf("expected revokeRefreshTokenByHash to be called")
	}
}

func TestLogoutHandler_ValidationErrors(t *testing.T) {
	authSvc := service.NewAuthService(&mockQuerier{}, testJWTSecret)
	handler := rest.NewLogoutHandler(authSvc)

	// Missing refresh_token
	body := `{}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 Bad Request for missing refresh_token", rec.Code)
	}

	// Malformed JSON
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", bytes.NewBufferString(`{bad`))
	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 Bad Request for malformed JSON", rec2.Code)
	}
}

func TestLogoutHandler_MethodNotAllowed(t *testing.T) {
	authSvc := service.NewAuthService(&mockQuerier{}, testJWTSecret)
	handler := rest.NewLogoutHandler(authSvc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/logout", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405 Method Not Allowed", rec.Code)
	}
}

func TestLogoutHandler_DatabaseError(t *testing.T) {
	mock := &mockQuerier{
		revokeRefreshTokenByHashFunc: func(ctx context.Context, arg store.RevokeRefreshTokenByHashParams) error {
			return errors.New("database connection failed")
		},
	}

	authSvc := service.NewAuthService(mock, testJWTSecret)
	handler := rest.NewLogoutHandler(authSvc)

	body := `{"refresh_token": "valid-raw-refresh-token"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 Internal Server Error, got %d", rec.Code, rec.Code)
	}
}

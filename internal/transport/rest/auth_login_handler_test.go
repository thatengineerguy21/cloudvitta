package rest_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

	dbtx := &testDBTX{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) pgx.Row {
			if strings.Contains(sql, "FROM users") {
				return &testRow{
					scanFn: func(dest ...interface{}) error {
						if idPtr, ok := dest[0].(*pgtype.UUID); ok {
							*idPtr = store.UUIDToPg(userUUID)
						}
						if emailPtr, ok := dest[1].(*string); ok {
							*emailPtr = email
						}
						if hashPtr, ok := dest[2].(*string); ok {
							*hashPtr = passwordHash
						}
						if createdPtr, ok := dest[3].(*pgtype.Timestamptz); ok {
							*createdPtr = store.TimestamptzFromTime(time.Now().UTC())
						}
						return nil
					},
				}
			}
			if strings.Contains(sql, "INSERT INTO refresh_tokens") {
				return &testRow{
					scanFn: func(dest ...interface{}) error {
						if idPtr, ok := dest[0].(*pgtype.UUID); ok {
							*idPtr = store.UUIDToPg(uuid.New())
						}
						return nil
					},
				}
			}
			return &testRow{}
		},
	}

	queries := store.New(dbtx)
	authSvc := service.NewAuthService(queries, testJWTSecret)
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

	dbtx := &testDBTX{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) pgx.Row {
			return &testRow{
				scanFn: func(dest ...interface{}) error {
					if idPtr, ok := dest[0].(*pgtype.UUID); ok {
						*idPtr = store.UUIDToPg(uuid.New())
					}
					if emailPtr, ok := dest[1].(*string); ok {
						*emailPtr = "user@example.com"
					}
					if hashPtr, ok := dest[2].(*string); ok {
						*hashPtr = passwordHash
					}
					return nil
				},
			}
		},
	}

	queries := store.New(dbtx)
	authSvc := service.NewAuthService(queries, testJWTSecret)
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
	dbtx := &testDBTX{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) pgx.Row {
			return &testRow{
				scanFn: func(dest ...interface{}) error {
					return pgx.ErrNoRows
				},
			}
		},
	}

	queries := store.New(dbtx)
	authSvc := service.NewAuthService(queries, testJWTSecret)
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
	queries := store.New(&testDBTX{})
	authSvc := service.NewAuthService(queries, testJWTSecret)
	handler := rest.NewLoginHandler(authSvc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/login", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405 Method Not Allowed", rec.Code)
	}
}

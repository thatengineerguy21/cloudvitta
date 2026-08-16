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
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/thatengineerguy21/CloudVitta/internal/service"
	"github.com/thatengineerguy21/CloudVitta/internal/store"
	"github.com/thatengineerguy21/CloudVitta/internal/transport/rest"
	"github.com/thatengineerguy21/CloudVitta/internal/transport/rest/middleware"
)

var testJWTSecret = []byte("super-secret-jwt-key-with-at-least-32-bytes-length!")

// testRow implements pgx.Row for mocked query responses.
type testRow struct {
	scanFn func(dest ...interface{}) error
}

func (r *testRow) Scan(dest ...interface{}) error {
	if r.scanFn != nil {
		return r.scanFn(dest...)
	}
	return pgx.ErrNoRows
}

// testDBTX implements store.DBTX for testing handlers with *store.Queries.
type testDBTX struct {
	queryRowFn func(ctx context.Context, sql string, args ...interface{}) pgx.Row
}

func (t *testDBTX) Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (t *testDBTX) Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error) {
	return nil, nil
}

func (t *testDBTX) QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row {
	if t.queryRowFn != nil {
		return t.queryRowFn(ctx, sql, args...)
	}
	return &testRow{}
}

func TestSignupHandler_Success(t *testing.T) {
	userUUID := uuid.New()
	dbtx := &testDBTX{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) pgx.Row {
			argEmail, _ := args[0].(string)
			argHash, _ := args[1].(string)
			return &testRow{
				scanFn: func(dest ...interface{}) error {
					if idPtr, ok := dest[0].(*pgtype.UUID); ok {
						*idPtr = store.UUIDToPg(userUUID)
					}
					if emailPtr, ok := dest[1].(*string); ok {
						*emailPtr = argEmail
					}
					if hashPtr, ok := dest[2].(*string); ok {
						*hashPtr = argHash
					}
					if createdPtr, ok := dest[3].(*pgtype.Timestamptz); ok {
						*createdPtr = store.TimestamptzFromTime(time.Now().UTC())
					}
					return nil
				},
			}
		},
	}

	queries := store.New(dbtx)
	authSvc := service.NewAuthService(queries, testJWTSecret)
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
	queries := store.New(&testDBTX{})
	authSvc := service.NewAuthService(queries, testJWTSecret)
	handler := rest.NewSignupHandler(authSvc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/signup", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405 Method Not Allowed", rec.Code)
	}
}

func TestSignupHandler_MalformedJSON(t *testing.T) {
	queries := store.New(&testDBTX{})
	authSvc := service.NewAuthService(queries, testJWTSecret)
	handler := rest.NewSignupHandler(authSvc)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/signup", bytes.NewBufferString(`{invalid-json`))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 Bad Request", rec.Code)
	}
}

func TestSignupHandler_ShortPassword(t *testing.T) {
	queries := store.New(&testDBTX{})
	authSvc := service.NewAuthService(queries, testJWTSecret)
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
	dbtx := &testDBTX{
		queryRowFn: func(ctx context.Context, sql string, args ...interface{}) pgx.Row {
			return &testRow{
				scanFn: func(dest ...interface{}) error {
					return &pgconn.PgError{
						Code:           "23505",
						ConstraintName: "users_email_key",
					}
				},
			}
		},
	}

	queries := store.New(dbtx)
	authSvc := service.NewAuthService(queries, testJWTSecret)
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

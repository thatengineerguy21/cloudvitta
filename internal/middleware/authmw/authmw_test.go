package authmw_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/thatengineerguy21/CloudVitta/internal/auth"
	"github.com/thatengineerguy21/CloudVitta/internal/middleware/authmw"
)

var testJWTSecret = []byte("super-secret-jwt-signing-key-32b-length!")

func TestAuthMiddleware_ValidToken(t *testing.T) {
	userID := uuid.New()
	fixedTime := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return fixedTime }

	token, err := auth.GenerateAccessToken(userID, "standard", testJWTSecret, fixedTime, 15*time.Minute)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	var capturedCtx authmw.AuthContext
	var capturedOk bool

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx, capturedOk = authmw.AuthFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	mw := authmw.NewAuthMiddleware(testJWTSecret, clock)
	ts := httptest.NewServer(mw(handler))
	defer ts.Close()

	req, err := http.NewRequest(http.MethodGet, ts.URL, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
	if !capturedOk {
		t.Fatal("expected AuthContext in request context, got none")
	}
	if !capturedCtx.IsAuth {
		t.Errorf("expected IsAuth true, got false")
	}
	if capturedCtx.UserID != userID.String() {
		t.Errorf("expected UserID %s, got %s", userID.String(), capturedCtx.UserID)
	}
	if capturedCtx.Tier != "standard" {
		t.Errorf("expected Tier 'standard', got %s", capturedCtx.Tier)
	}
}

func TestAuthMiddleware_ExpiredToken(t *testing.T) {
	userID := uuid.New()
	mintTime := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	verifyTime := mintTime.Add(30 * time.Minute) // Token expired
	clock := func() time.Time { return verifyTime }

	token, err := auth.GenerateAccessToken(userID, "standard", testJWTSecret, mintTime, 15*time.Minute)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	var capturedCtx authmw.AuthContext
	var capturedOk bool

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx, capturedOk = authmw.AuthFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	mw := authmw.NewAuthMiddleware(testJWTSecret, clock)
	ts := httptest.NewServer(mw(handler))
	defer ts.Close()

	req, err := http.NewRequest(http.MethodGet, ts.URL, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
	if capturedOk && capturedCtx.IsAuth {
		t.Errorf("expected unauthenticated context, got %+v", capturedCtx)
	}
}

func TestAuthMiddleware_MissingHeader(t *testing.T) {
	var capturedCtx authmw.AuthContext
	var capturedOk bool

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx, capturedOk = authmw.AuthFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	mw := authmw.NewAuthMiddleware(testJWTSecret, time.Now)
	ts := httptest.NewServer(mw(handler))
	defer ts.Close()

	req, err := http.NewRequest(http.MethodGet, ts.URL, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
	if capturedOk && capturedCtx.IsAuth {
		t.Errorf("expected unauthenticated context, got %+v", capturedCtx)
	}
}

func TestAuthMiddleware_MalformedHeader(t *testing.T) {
	var capturedCtx authmw.AuthContext
	var capturedOk bool

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedCtx, capturedOk = authmw.AuthFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	mw := authmw.NewAuthMiddleware(testJWTSecret, time.Now)
	ts := httptest.NewServer(mw(handler))
	defer ts.Close()

	req, err := http.NewRequest(http.MethodGet, ts.URL, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Authorization", "NotBearer malformedtoken123")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
	if capturedOk && capturedCtx.IsAuth {
		t.Errorf("expected unauthenticated context, got %+v", capturedCtx)
	}
}

func TestRequireAuth_Authenticated(t *testing.T) {
	userID := uuid.New()
	fixedTime := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return fixedTime }

	token, err := auth.GenerateAccessToken(userID, "standard", testJWTSecret, fixedTime, 15*time.Minute)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	mw := authmw.NewAuthMiddleware(testJWTSecret, clock)
	guarded := authmw.RequireAuth(handler)
	ts := httptest.NewServer(mw(guarded))
	defer ts.Close()

	req, err := http.NewRequest(http.MethodPost, ts.URL+"/mcp", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
	if !handlerCalled {
		t.Errorf("expected downstream handler to be called")
	}
}

func TestRequireAuth_Unauthenticated(t *testing.T) {
	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	mw := authmw.NewAuthMiddleware(testJWTSecret, time.Now)
	guarded := authmw.RequireAuth(handler)
	ts := httptest.NewServer(mw(guarded))
	defer ts.Close()

	req, err := http.NewRequest(http.MethodPost, ts.URL+"/mcp", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected status 401 Unauthorized, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/problem+json" {
		t.Errorf("expected Content-Type application/problem+json, got %s", ct)
	}
	if handlerCalled {
		t.Errorf("expected downstream handler NOT to be called")
	}
}

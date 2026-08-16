package ratelimit_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/thatengineerguy21/CloudVitta/internal/auth"
	"github.com/thatengineerguy21/CloudVitta/internal/middleware/authmw"
	"github.com/thatengineerguy21/CloudVitta/internal/middleware/ratelimit"
)

var testCookieSecret = []byte("super-secret-cookie-signing-key-32b!")

func TestExtractClientIP(t *testing.T) {
	tests := []struct {
		name       string
		headers    map[string]string
		remoteAddr string
		wantIP     string
	}{
		{
			name:       "Single XFF IP",
			headers:    map[string]string{"X-Forwarded-For": "203.0.113.195"},
			remoteAddr: "127.0.0.1:12345",
			wantIP:     "203.0.113.195",
		},
		{
			name:       "Multiple XFF IPs chooses last element (trusted proxy)",
			headers:    map[string]string{"X-Forwarded-For": "198.51.100.1, 203.0.113.195"},
			remoteAddr: "127.0.0.1:12345",
			wantIP:     "203.0.113.195",
		},
		{
			name:       "X-Forwarded-For chain takes last element",
			headers:    map[string]string{"X-Forwarded-For": "1.2.3.4, 5.6.7.8, 9.10.11.12"},
			remoteAddr: "10.0.0.1:1234",
			wantIP:     "9.10.11.12",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			for k, v := range tc.headers {
				req.Header.Set(k, v)
			}
			req.RemoteAddr = tc.remoteAddr

			ip := ratelimit.ExtractClientIP(req)
			if ip != tc.wantIP {
				t.Errorf("expected %q, got %q", tc.wantIP, ip)
			}
		})
	}
}

func setupTestRateLimiter(t *testing.T, cfg ratelimit.Config) (*ratelimit.RateLimiter, *miniredis.Miniredis, *redis.Client) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	if cfg.CookieSecret == nil {
		cfg.CookieSecret = testCookieSecret
	}
	if cfg.StandardTierRate == 0 {
		cfg.StandardTierRate = 120
	}
	if cfg.FreeTierRate == 0 {
		cfg.FreeTierRate = 20
	}
	if cfg.IPCeilingRate == 0 {
		cfg.IPCeilingRate = 60
	}

	rl := ratelimit.NewRateLimiter(rdb, cfg)
	return rl, mr, rdb
}

func TestRateLimiter_AuthenticatedUser_StandardTier(t *testing.T) {
	fixedTime := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	rl, mr, rdb := setupTestRateLimiter(t, ratelimit.Config{
		StandardTierRate: 120,
		FreeTierRate:     20,
		IPCeilingRate:    200,
		Clock:            func() time.Time { return fixedTime },
	})
	defer mr.Close()
	defer func() { _ = rdb.Close() }()

	userID := uuid.New().String()
	handler := rl.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/prices/compute", nil)
	req.Header.Set("X-Forwarded-For", "198.51.100.1")
	// Inject auth context
	ctx := authmw.ContextWithAuth(req.Context(), authmw.AuthContext{
		UserID: userID,
		Tier:   "standard",
		IsAuth: true,
	})
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 OK", rec.Code)
	}
	if limit := rec.Header().Get("X-RateLimit-Limit"); limit != "120" {
		t.Errorf("expected X-RateLimit-Limit 120, got %s", limit)
	}
	if remaining := rec.Header().Get("X-RateLimit-Remaining"); remaining != "119" {
		t.Errorf("expected X-RateLimit-Remaining 119, got %s", remaining)
	}

	// Verify Redis key format
	minWindow := fixedTime.Unix() / 60
	expectedKey := fmt.Sprintf("ratelimit:user:%s:%d", userID, minWindow)
	val, err := rdb.Get(context.Background(), expectedKey).Result()
	if err != nil {
		t.Fatalf("expected key %s in Redis, got err: %v", expectedKey, err)
	}
	if val != "1" {
		t.Errorf("expected key value '1', got %s", val)
	}
}

func TestRateLimiter_AnonymousCookie_FreeTier(t *testing.T) {
	fixedTime := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	rl, mr, rdb := setupTestRateLimiter(t, ratelimit.Config{
		StandardTierRate: 120,
		FreeTierRate:     20,
		IPCeilingRate:    60,
		Clock:            func() time.Time { return fixedTime },
	})
	defer mr.Close()
	defer func() { _ = rdb.Close() }()

	cookie, err := auth.MintAnonCookie(testCookieSecret, fixedTime, false)
	if err != nil {
		t.Fatalf("failed to mint cookie: %v", err)
	}
	anonID, err := auth.VerifyAnonCookie(cookie.Value, testCookieSecret, fixedTime)
	if err != nil {
		t.Fatalf("failed to verify cookie: %v", err)
	}

	handler := rl.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/prices/compute", nil)
	req.Header.Set("X-Forwarded-For", "198.51.100.10")
	req.AddCookie(cookie)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 OK", rec.Code)
	}
	if limit := rec.Header().Get("X-RateLimit-Limit"); limit != "20" {
		t.Errorf("expected X-RateLimit-Limit 20, got %s", limit)
	}
	if remaining := rec.Header().Get("X-RateLimit-Remaining"); remaining != "19" {
		t.Errorf("expected X-RateLimit-Remaining 19, got %s", remaining)
	}

	// Verify Redis key format
	minWindow := fixedTime.Unix() / 60
	expectedKey := fmt.Sprintf("ratelimit:anon:%s:%d", anonID, minWindow)
	val, err := rdb.Get(context.Background(), expectedKey).Result()
	if err != nil {
		t.Fatalf("expected key %s in Redis, got err: %v", expectedKey, err)
	}
	if val != "1" {
		t.Errorf("expected key value '1', got %s", val)
	}
}

func TestRateLimiter_FallbackIP_MintsCookie(t *testing.T) {
	fixedTime := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	rl, mr, rdb := setupTestRateLimiter(t, ratelimit.Config{
		StandardTierRate: 120,
		FreeTierRate:     20,
		IPCeilingRate:    60,
		Clock:            func() time.Time { return fixedTime },
	})
	defer mr.Close()
	defer func() { _ = rdb.Close() }()

	handler := rl.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/prices/compute", nil)
	req.Header.Set("X-Forwarded-For", "198.51.100.25")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 OK", rec.Code)
	}
	if limit := rec.Header().Get("X-RateLimit-Limit"); limit != "20" {
		t.Errorf("expected X-RateLimit-Limit 20, got %s", limit)
	}

	// Check that a fresh Set-Cookie header was issued
	cookies := rec.Result().Cookies()
	var anonCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == auth.AnonCookieName {
			anonCookie = c
			break
		}
	}
	if anonCookie == nil {
		t.Fatal("expected Set-Cookie cv_anon_id header, got none")
	}

	// Verify the issued cookie is valid
	if _, err := auth.VerifyAnonCookie(anonCookie.Value, testCookieSecret, fixedTime); err != nil {
		t.Errorf("issued cookie failed validation: %v", err)
	}

	// Verify Redis key format is ip-based
	minWindow := fixedTime.Unix() / 60
	expectedKey := fmt.Sprintf("ratelimit:ip:198.51.100.25:%d", minWindow)
	val, err := rdb.Get(context.Background(), expectedKey).Result()
	if err != nil {
		t.Fatalf("expected key %s in Redis, got err: %v", expectedKey, err)
	}
	if val != "1" {
		t.Errorf("expected key value '1', got %s", val)
	}
}

func TestRateLimiter_BluntIPCeiling_Enforced(t *testing.T) {
	fixedTime := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	rl, mr, rdb := setupTestRateLimiter(t, ratelimit.Config{
		StandardTierRate: 120,
		FreeTierRate:     20,
		IPCeilingRate:    5, // Low ceiling to test quickly
		Clock:            func() time.Time { return fixedTime },
	})
	defer mr.Close()
	defer func() { _ = rdb.Close() }()

	handler := rl.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	clientIP := "198.51.100.50"

	// Simulate attacker cycling different cookie IDs from the exact same IP
	for i := 1; i <= 5; i++ {
		cookie, err := auth.MintAnonCookie(testCookieSecret, fixedTime, false)
		if err != nil {
			t.Fatalf("failed to mint cookie: %v", err)
		}

		req := httptest.NewRequest(http.MethodGet, "/api/v1/prices/compute", nil)
		req.Header.Set("X-Forwarded-For", clientIP)
		req.AddCookie(cookie)

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("request %d status = %d, want 200 OK", i, rec.Code)
		}
	}

	// 6th request with yet another cookie from same IP should be blocked by blunt IP ceiling
	freshCookie, err := auth.MintAnonCookie(testCookieSecret, fixedTime, false)
	if err != nil {
		t.Fatalf("failed to mint cookie: %v", err)
	}

	req6 := httptest.NewRequest(http.MethodGet, "/api/v1/prices/compute", nil)
	req6.Header.Set("X-Forwarded-For", clientIP)
	req6.AddCookie(freshCookie)

	rec6 := httptest.NewRecorder()
	handler.ServeHTTP(rec6, req6)

	if rec6.Code != http.StatusTooManyRequests {
		t.Fatalf("request 6 status = %d, want 429 Too Many Requests (IP ceiling)", rec6.Code)
	}
	if retryAfter := rec6.Header().Get("Retry-After"); retryAfter == "" {
		t.Error("expected Retry-After header on 429 response")
	}
}

func TestRateLimiter_WithProfile(t *testing.T) {
	fixedTime := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	rl, mr, rdb := setupTestRateLimiter(t, ratelimit.Config{
		StandardTierRate: 120,
		FreeTierRate:     20,
		IPCeilingRate:    60,
		Clock:            func() time.Time { return fixedTime },
	})
	defer mr.Close()
	defer func() { _ = rdb.Close() }()

	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Login profile limit = 2
	loginHandler := rl.WithProfile("login", 2)(dummyHandler)

	for i := 1; i <= 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
		req.Header.Set("X-Forwarded-For", "198.51.100.42")
		rec := httptest.NewRecorder()
		loginHandler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("login request %d status = %d, want 200 OK", i, rec.Code)
		}
	}

	// 3rd login request should be 429
	req3 := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	req3.Header.Set("X-Forwarded-For", "198.51.100.42")
	rec3 := httptest.NewRecorder()
	loginHandler.ServeHTTP(rec3, req3)

	if rec3.Code != http.StatusTooManyRequests {
		t.Fatalf("login request 3 status = %d, want 429 Too Many Requests", rec3.Code)
	}
}

func TestRateLimiter_RateLimitExceeded_FreeTier(t *testing.T) {
	fixedTime := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	rl, mr, rdb := setupTestRateLimiter(t, ratelimit.Config{
		StandardTierRate: 120,
		FreeTierRate:     2,
		IPCeilingRate:    10,
		Clock:            func() time.Time { return fixedTime },
	})
	defer mr.Close()
	defer func() { _ = rdb.Close() }()

	cookie, _ := auth.MintAnonCookie(testCookieSecret, fixedTime, false)
	handler := rl.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for i := 1; i <= 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/prices/compute", nil)
		req.Header.Set("X-Forwarded-For", "198.51.100.99")
		req.AddCookie(cookie)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("request %d status = %d, want 200 OK", i, rec.Code)
		}
	}

	// 3rd request with same cookie should be 429
	req3 := httptest.NewRequest(http.MethodGet, "/api/v1/prices/compute", nil)
	req3.Header.Set("X-Forwarded-For", "198.51.100.99")
	req3.AddCookie(cookie)
	rec3 := httptest.NewRecorder()
	handler.ServeHTTP(rec3, req3)

	if rec3.Code != http.StatusTooManyRequests {
		t.Fatalf("request 3 status = %d, want 429 Too Many Requests", rec3.Code)
	}
	if rec3.Header().Get("Retry-After") == "" {
		t.Error("expected Retry-After header on 429")
	}
	if rec3.Header().Get("X-RateLimit-Limit") != "2" {
		t.Errorf("expected X-RateLimit-Limit 2, got %s", rec3.Header().Get("X-RateLimit-Limit"))
	}
	if rec3.Header().Get("X-RateLimit-Remaining") != "0" {
		t.Errorf("expected X-RateLimit-Remaining 0, got %s", rec3.Header().Get("X-RateLimit-Remaining"))
	}
	resetVal, err := strconv.ParseInt(rec3.Header().Get("X-RateLimit-Reset"), 10, 64)
	if err != nil || resetVal <= fixedTime.Unix() {
		t.Errorf("expected valid future reset timestamp, got %s", rec3.Header().Get("X-RateLimit-Reset"))
	}
}

func TestRateLimiter_AuthenticatedUser_BypassesIPCeiling(t *testing.T) {
	fixedTime := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	// Standard rate is 10, but IP ceiling is only 3
	rl, mr, rdb := setupTestRateLimiter(t, ratelimit.Config{
		StandardTierRate: 10,
		FreeTierRate:     2,
		IPCeilingRate:    3,
		Clock:            func() time.Time { return fixedTime },
	})
	defer mr.Close()
	defer func() { _ = rdb.Close() }()

	userID := uuid.New().String()
	handler := rl.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	clientIP := "198.51.100.77"

	// Authenticated user makes 6 requests from the same IP (which is greater than IPCeilingRate=3)
	for i := 1; i <= 6; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/prices/compute", nil)
		req.Header.Set("X-Forwarded-For", clientIP)
		ctx := authmw.ContextWithAuth(req.Context(), authmw.AuthContext{
			UserID: userID,
			Tier:   "standard",
			IsAuth: true,
		})
		req = req.WithContext(ctx)

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("request %d failed with status %d (should not be blocked by IP ceiling of 3)", i, rec.Code)
		}
		if limit := rec.Header().Get("X-RateLimit-Limit"); limit != "10" {
			t.Errorf("expected X-RateLimit-Limit 10, got %s", limit)
		}
	}
}

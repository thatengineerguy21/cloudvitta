package ratelimit_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/thatengineerguy21/CloudVitta/internal/middleware/ratelimit"
)

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

func TestRateLimiter(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	rl := ratelimit.NewRateLimiter(rdb, 2) // limit to 2 requests/min

	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handlerToTest := rl.Handler(dummyHandler)

	for i := 1; i <= 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/prices/compute", nil)
		req.Header.Set("X-Forwarded-For", "198.51.100.42")
		rec := httptest.NewRecorder()

		handlerToTest.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("request %d status = %d, want 200 OK", i, rec.Code)
		}
	}

	// 3rd request should be blocked with 429
	req := httptest.NewRequest(http.MethodGet, "/api/v1/prices/compute", nil)
	req.Header.Set("X-Forwarded-For", "198.51.100.42")
	rec := httptest.NewRecorder()

	handlerToTest.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("request 3 status = %d, want 429 Too Many Requests", rec.Code)
	}

	retryAfter := rec.Header().Get("Retry-After")
	if retryAfter == "" {
		t.Errorf("expected Retry-After header on 429 response")
	}
}

func TestRateLimiter_WithProfile(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	rl := ratelimit.NewRateLimiter(rdb, 60) // generic limit = 60

	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Login profile limit = 1
	loginHandler := rl.WithProfile("login", 1)(dummyHandler)

	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	req1.Header.Set("X-Forwarded-For", "198.51.100.42")
	rec1 := httptest.NewRecorder()
	loginHandler.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusOK {
		t.Fatalf("login request 1 status = %d, want 200 OK", rec1.Code)
	}

	// 2nd login request should be 429
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	req2.Header.Set("X-Forwarded-For", "198.51.100.42")
	rec2 := httptest.NewRecorder()
	loginHandler.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("login request 2 status = %d, want 429 Too Many Requests", rec2.Code)
	}

	// Generic handler for same IP should still pass because profiles are partitioned
	genericHandler := rl.Handler(dummyHandler)
	reqGeneric := httptest.NewRequest(http.MethodGet, "/api/v1/calculate", nil)
	reqGeneric.Header.Set("X-Forwarded-For", "198.51.100.42")
	recGeneric := httptest.NewRecorder()
	genericHandler.ServeHTTP(recGeneric, reqGeneric)

	if recGeneric.Code != http.StatusOK {
		t.Errorf("generic request status = %d, want 200 OK (profile isolation)", recGeneric.Code)
	}
}

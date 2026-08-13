package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/thatengineerguy21/CloudVitta/internal/transport/rest/middleware"
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
			name:       "No XFF header falls back to RemoteAddr host",
			headers:    map[string]string{},
			remoteAddr: "192.0.2.1:54321",
			wantIP:     "192.0.2.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tt.remoteAddr
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			got := middleware.ExtractClientIP(req)
			if got != tt.wantIP {
				t.Errorf("ExtractClientIP() = %q, want %q", got, tt.wantIP)
			}
		})
	}
}

func TestRateLimiter_ExceedLimit(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	limiter := middleware.NewRateLimiter(rdb, 2) // limit to 2 req/min

	dummyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handlerToTest := limiter.Handler(dummyHandler)

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

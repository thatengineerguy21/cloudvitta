package rest_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
	"github.com/thatengineerguy21/CloudVitta/internal/auth"
	"github.com/thatengineerguy21/CloudVitta/internal/cache"
	"github.com/thatengineerguy21/CloudVitta/internal/config"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/service"
	"github.com/thatengineerguy21/CloudVitta/internal/transport/rest"
)

func TestRouter_Integration_TieredRateLimitingAndCORS(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	jwtSecret := "super-secret-jwt-key-with-at-least-32-bytes-length!"
	anonSecret := "super-secret-cookie-signing-key-32b!"

	cfg := &config.Config{
		Auth: config.AuthConfig{
			JWTSecret:        jwtSecret,
			AnonCookieSecret: anonSecret,
		},
		RateLimit: config.RateLimitConfig{
			StandardTierRate: 120,
			FreeTierRate:     20,
			IPCeilingRate:    60,
			LoginRate:        10,
		},
		CORS: config.CORSConfig{
			AllowedOrigins:   []string{"https://cloudvitta.dev"},
			AllowCredentials: true,
		},
	}

	pricingSvc := service.NewPricingService(nil, rdb)
	router := rest.NewRouter(pricingSvc, nil, nil, nil, nil, rdb, cfg)

	ts := httptest.NewServer(router)

	defer ts.Close()

	client := ts.Client()

	t.Run("Anonymous First Request issues cv_anon_id cookie, free tier limit, and permissive CORS on prices", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/prices/compute?vcpu=2&ram=4", nil)
		if err != nil {
			t.Fatalf("failed to create request: %v", err)
		}
		req.Header.Set("Origin", "https://thirdparty.org")
		req.Header.Set("X-Forwarded-For", "198.51.100.100")

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		// Verify Rate Limit headers
		if limit := resp.Header.Get("X-RateLimit-Limit"); limit != "20" {
			t.Errorf("expected X-RateLimit-Limit 20 for anonymous user, got %s", limit)
		}

		// Verify Permissive CORS on public read path
		if origin := resp.Header.Get("Access-Control-Allow-Origin"); origin != "*" {
			t.Errorf("expected permissive Access-Control-Allow-Origin '*', got %s", origin)
		}
		if creds := resp.Header.Get("Access-Control-Allow-Credentials"); creds != "" {
			t.Errorf("expected empty Access-Control-Allow-Credentials on public read path, got %s", creds)
		}

		// Verify cv_anon_id cookie issued
		var anonCookie *http.Cookie
		for _, c := range resp.Cookies() {
			if c.Name == auth.AnonCookieName {
				anonCookie = c
				break
			}
		}
		if anonCookie == nil {
			t.Fatal("expected Set-Cookie cv_anon_id header, got none")
		}

		// Subsequent request presenting cookie uses anon tier
		req2, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/prices/compute?vcpu=2&ram=4", nil)
		req2.Header.Set("Origin", "https://thirdparty.org")
		req2.Header.Set("X-Forwarded-For", "198.51.100.100")
		req2.AddCookie(anonCookie)

		resp2, err := client.Do(req2)
		if err != nil {
			t.Fatalf("subsequent request failed: %v", err)
		}
		defer func() { _ = resp2.Body.Close() }()

		if limit := resp2.Header.Get("X-RateLimit-Limit"); limit != "20" {
			t.Errorf("expected X-RateLimit-Limit 20 with anon cookie, got %s", limit)
		}
	})

	t.Run("Authenticated User receives standard tier limit of 120", func(t *testing.T) {
		userID := uuid.New()
		token, err := auth.GenerateAccessToken(userID, "standard", []byte(jwtSecret), time.Now(), 15*time.Minute)
		if err != nil {
			t.Fatalf("failed to generate token: %v", err)
		}

		req, err := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/prices/compute?vcpu=2&ram=4", nil)
		if err != nil {
			t.Fatalf("failed to create request: %v", err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Origin", "https://cloudvitta.dev")
		req.Header.Set("X-Forwarded-For", "198.51.100.101")

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if limit := resp.Header.Get("X-RateLimit-Limit"); limit != "120" {
			t.Errorf("expected X-RateLimit-Limit 120 for authenticated user, got %s", limit)
		}
	})

	t.Run("Narrowed CORS on Calculate with Allowed Origin", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/calculate", strings.NewReader(`{}`))
		if err != nil {
			t.Fatalf("failed to create request: %v", err)
		}
		req.Header.Set("Origin", "https://cloudvitta.dev")
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Forwarded-For", "198.51.100.102")

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if origin := resp.Header.Get("Access-Control-Allow-Origin"); origin != "https://cloudvitta.dev" {
			t.Errorf("expected reflected Access-Control-Allow-Origin, got %s", origin)
		}
		if creds := resp.Header.Get("Access-Control-Allow-Credentials"); creds != "true" {
			t.Errorf("expected Access-Control-Allow-Credentials true, got %s", creds)
		}
	})

	t.Run("Narrowed CORS on Calculate with Disallowed Origin", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/calculate", strings.NewReader(`{}`))
		if err != nil {
			t.Fatalf("failed to create request: %v", err)
		}
		req.Header.Set("Origin", "https://malicious-site.com")
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Forwarded-For", "198.51.100.103")

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if origin := resp.Header.Get("Access-Control-Allow-Origin"); origin != "" {
			t.Errorf("expected empty Access-Control-Allow-Origin for unauthorized origin on calculate, got %s", origin)
		}
	})

	t.Run("Preflight OPTIONS on calculate returns 204 and preflight headers", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodOptions, ts.URL+"/api/v1/calculate", nil)
		if err != nil {
			t.Fatalf("failed to create request: %v", err)
		}
		req.Header.Set("Origin", "https://cloudvitta.dev")
		req.Header.Set("Access-Control-Request-Method", "POST")

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusNoContent {
			t.Errorf("expected status 204, got %d", resp.StatusCode)
		}
		if origin := resp.Header.Get("Access-Control-Allow-Origin"); origin != "https://cloudvitta.dev" {
			t.Errorf("expected Access-Control-Allow-Origin 'https://cloudvitta.dev', got %s", origin)
		}
		if methods := resp.Header.Get("Access-Control-Allow-Methods"); !strings.Contains(methods, "POST") {
			t.Errorf("expected POST in Access-Control-Allow-Methods, got %s", methods)
		}
	})

	t.Run("GET /api/v1/prices/kubernetes returns 200 with rate limiting", func(t *testing.T) {
		k8sObs := []domain.PriceObservation{
			{
				Provider:        "aws",
				ServiceCategory: "kubernetes",
				SkuID:           "SKU-AWS-EKS-STD",
				Region:          "us-east-1",
				RegionGroup:     "us-east",
				PriceAmount:     decimal.RequireFromString("0.10"),
				PriceCurrency:   "USD",
				Unit:            "hour",
				KubernetesAttributes: domain.KubernetesAttributes{
					Tier: "standard",
				},
				FetchedAt: time.Now().UTC(),
			},
		}
		_ = cache.Warm(context.Background(), rdb, cache.BuildKey(cache.SchemaVersion, "aws", "kubernetes", "us-east-1"), k8sObs, time.Hour)

		req, err := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/prices/kubernetes?tier=standard", nil)
		if err != nil {
			t.Fatalf("failed to create request: %v", err)
		}
		req.Header.Set("Origin", "https://cloudvitta.dev")

		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}
		if limit := resp.Header.Get("X-RateLimit-Limit"); limit == "" {
			t.Errorf("expected X-RateLimit-Limit header, got empty")
		}
	})
}

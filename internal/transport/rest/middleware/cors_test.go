package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/thatengineerguy21/CloudVitta/internal/config"
	"github.com/thatengineerguy21/CloudVitta/internal/transport/rest/middleware"
)

func TestCORS_AllowedOrigin_ReflectedWithCredentials(t *testing.T) {
	cfg := config.CORSConfig{
		AllowedOrigins:   []string{"https://cloudvitta.dev", "http://localhost:3000"},
		AllowCredentials: true,
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	corsMw := middleware.NewCORSMiddleware(cfg)
	ts := httptest.NewServer(corsMw.Handler(handler))
	defer ts.Close()

	req, err := http.NewRequest(http.MethodGet, ts.URL, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Origin", "https://cloudvitta.dev")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
	if origin := resp.Header.Get("Access-Control-Allow-Origin"); origin != "https://cloudvitta.dev" {
		t.Errorf("expected Access-Control-Allow-Origin 'https://cloudvitta.dev', got '%s'", origin)
	}
	if creds := resp.Header.Get("Access-Control-Allow-Credentials"); creds != "true" {
		t.Errorf("expected Access-Control-Allow-Credentials 'true', got '%s'", creds)
	}
	if vary := resp.Header.Get("Vary"); vary != "Origin" {
		t.Errorf("expected Vary 'Origin', got '%s'", vary)
	}
}

func TestCORS_DisallowedOrigin_NoCORSHeaders(t *testing.T) {
	cfg := config.CORSConfig{
		AllowedOrigins:   []string{"https://cloudvitta.dev"},
		AllowCredentials: true,
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	corsMw := middleware.NewCORSMiddleware(cfg)
	ts := httptest.NewServer(corsMw.Handler(handler))
	defer ts.Close()

	req, err := http.NewRequest(http.MethodGet, ts.URL, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Origin", "https://malicious-site.com")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
	if origin := resp.Header.Get("Access-Control-Allow-Origin"); origin != "" {
		t.Errorf("expected empty Access-Control-Allow-Origin for unauthorized origin, got '%s'", origin)
	}
	if creds := resp.Header.Get("Access-Control-Allow-Credentials"); creds != "" {
		t.Errorf("expected empty Access-Control-Allow-Credentials for unauthorized origin, got '%s'", creds)
	}
}

func TestCORS_PreflightOptions(t *testing.T) {
	cfg := config.CORSConfig{
		AllowedOrigins:   []string{"https://cloudvitta.dev"},
		AllowCredentials: true,
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("downstream handler should not be called for OPTIONS preflight")
	})

	corsMw := middleware.NewCORSMiddleware(cfg)
	ts := httptest.NewServer(corsMw.Handler(handler))
	defer ts.Close()

	req, err := http.NewRequest(http.MethodOptions, ts.URL+"/api/v1/calculate", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Origin", "https://cloudvitta.dev")
	req.Header.Set("Access-Control-Request-Method", "POST")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("expected status 204 No Content, got %d", resp.StatusCode)
	}
	if origin := resp.Header.Get("Access-Control-Allow-Origin"); origin != "https://cloudvitta.dev" {
		t.Errorf("expected Access-Control-Allow-Origin 'https://cloudvitta.dev', got '%s'", origin)
	}
	if methods := resp.Header.Get("Access-Control-Allow-Methods"); methods == "" {
		t.Error("expected Access-Control-Allow-Methods header")
	}
	if headers := resp.Header.Get("Access-Control-Allow-Headers"); headers == "" {
		t.Error("expected Access-Control-Allow-Headers header")
	}
	if maxAge := resp.Header.Get("Access-Control-Max-Age"); maxAge != "86400" {
		t.Errorf("expected Access-Control-Max-Age '86400', got '%s'", maxAge)
	}
}

func TestCORS_WildcardSubdomain(t *testing.T) {
	cfg := config.CORSConfig{
		AllowedOrigins:   []string{"https://*.cloudvitta.dev", "http://*.localhost"},
		AllowCredentials: true,
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	corsMw := middleware.NewCORSMiddleware(cfg)
	ts := httptest.NewServer(corsMw.Handler(handler))
	defer ts.Close()

	testCases := []struct {
		origin  string
		allowed bool
	}{
		{"https://app.cloudvitta.dev", true},
		{"https://api.cloudvitta.dev", true},
		{"https://staging.app.cloudvitta.dev", true},
		{"http://test.localhost", true},
		{"https://otherdomain.com", false},
		{"http://cloudvitta.dev", false}, // wrong scheme (http vs https://*.cloudvitta.dev)
	}

	for _, tc := range testCases {
		t.Run(tc.origin, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, ts.URL, nil)
			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}
			req.Header.Set("Origin", tc.origin)

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer func() { _ = resp.Body.Close() }()

			originHeader := resp.Header.Get("Access-Control-Allow-Origin")
			if tc.allowed && originHeader != tc.origin {
				t.Errorf("expected origin %s to be allowed, got '%s'", tc.origin, originHeader)
			} else if !tc.allowed && originHeader != "" {
				t.Errorf("expected origin %s to be rejected, got '%s'", tc.origin, originHeader)
			}
		})
	}
}

func TestCORS_PublicReadPath_PermissiveWithoutCredentials(t *testing.T) {
	cfg := config.CORSConfig{
		AllowedOrigins:   []string{"https://cloudvitta.dev"},
		AllowCredentials: true,
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("public price data"))
	})

	corsMw := middleware.NewCORSMiddleware(cfg)
	ts := httptest.NewServer(corsMw.Handler(handler))
	defer ts.Close()

	// Calling a public prices endpoint from an arbitrary third-party origin
	req, err := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/prices/compute", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Origin", "https://third-party-aggregator.com")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
	if origin := resp.Header.Get("Access-Control-Allow-Origin"); origin != "*" {
		t.Errorf("expected Access-Control-Allow-Origin '*', got '%s'", origin)
	}
	if creds := resp.Header.Get("Access-Control-Allow-Credentials"); creds != "" {
		t.Errorf("expected empty Access-Control-Allow-Credentials for public read path, got '%s'", creds)
	}
}

func TestCORS_CredentialedPath_NarrowedWithCredentials(t *testing.T) {
	cfg := config.CORSConfig{
		AllowedOrigins:   []string{"https://cloudvitta.dev"},
		AllowCredentials: true,
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	corsMw := middleware.NewCORSMiddleware(cfg)
	ts := httptest.NewServer(corsMw.Handler(handler))
	defer ts.Close()

	// 1. Allowed origin on calculate endpoint
	reqAllowed, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/calculate", nil)
	reqAllowed.Header.Set("Origin", "https://cloudvitta.dev")

	respAllowed, err := http.DefaultClient.Do(reqAllowed)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = respAllowed.Body.Close() }()

	if origin := respAllowed.Header.Get("Access-Control-Allow-Origin"); origin != "https://cloudvitta.dev" {
		t.Errorf("expected Access-Control-Allow-Origin 'https://cloudvitta.dev', got '%s'", origin)
	}
	if creds := respAllowed.Header.Get("Access-Control-Allow-Credentials"); creds != "true" {
		t.Errorf("expected Access-Control-Allow-Credentials 'true', got '%s'", creds)
	}

	// 2. Disallowed origin on calculate endpoint
	reqDisallowed, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/calculate", nil)
	reqDisallowed.Header.Set("Origin", "https://unauthorized-domain.com")

	respDisallowed, err := http.DefaultClient.Do(reqDisallowed)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer func() { _ = respDisallowed.Body.Close() }()

	if origin := respDisallowed.Header.Get("Access-Control-Allow-Origin"); origin != "" {
		t.Errorf("expected empty Access-Control-Allow-Origin for unauthorized origin on credentialed path, got '%s'", origin)
	}
	if creds := respDisallowed.Header.Get("Access-Control-Allow-Credentials"); creds != "" {
		t.Errorf("expected empty Access-Control-Allow-Credentials, got '%s'", creds)
	}
}

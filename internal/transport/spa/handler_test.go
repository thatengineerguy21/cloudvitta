package spa_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/thatengineerguy21/CloudVitta/internal/transport/spa"
)

func mockSPAFileSystem() fstest.MapFS {
	return fstest.MapFS{
		"index.html": &fstest.MapFile{
			Data: []byte("<!doctype html><html><head><title>CloudVitta Test</title></head><body><div id=\"root\">Mock Root</div></body></html>"),
		},
		"assets/index-abc1234.js": &fstest.MapFile{
			Data: []byte("console.log('cloudvitta bundle');"),
		},
		"assets/theme-xyz5678.css": &fstest.MapFile{
			Data: []byte("body { background: #0b0f17; }"),
		},
		"favicon.svg": &fstest.MapFile{
			Data: []byte("<svg xmlns=\"http://www.w3.org/2000/svg\"><circle r=\"10\"/></svg>"),
		},
	}
}

func TestSPAHandler_RootAndSPARoutes(t *testing.T) {
	handler := spa.NewHandlerWithFS(mockSPAFileSystem())

	routes := []string{
		"/",
		"/compare/compute",
		"/compare/storage",
		"/compare/network",
		"/compare/database",
		"/compare/database-nosql",
		"/compare/kubernetes",
		"/compare/serverless",
		"/calculate",
		"/status",
		"/non-existent-page",
	}

	for _, route := range routes {
		t.Run(route, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, route, nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected status 200 OK for route %s, got %d", route, rec.Code)
			}

			// Verify Content-Type
			contentType := rec.Header().Get("Content-Type")
			if !strings.Contains(contentType, "text/html") {
				t.Errorf("expected Content-Type containing text/html, got %q", contentType)
			}

			// Verify Cache-Control
			cacheControl := rec.Header().Get("Cache-Control")
			if cacheControl != "no-cache, no-store, must-revalidate" {
				t.Errorf("expected Cache-Control 'no-cache, no-store, must-revalidate', got %q", cacheControl)
			}

			// Verify Security Headers
			if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
				t.Errorf("expected X-Content-Type-Options: nosniff, got %q", rec.Header().Get("X-Content-Type-Options"))
			}
			if rec.Header().Get("X-Frame-Options") != "DENY" {
				t.Errorf("expected X-Frame-Options: DENY, got %q", rec.Header().Get("X-Frame-Options"))
			}
			if rec.Header().Get("Referrer-Policy") != "strict-origin-when-cross-origin" {
				t.Errorf("expected Referrer-Policy: strict-origin-when-cross-origin, got %q", rec.Header().Get("Referrer-Policy"))
			}

			// Verify Body Content
			body := rec.Body.String()
			if !strings.Contains(body, "Mock Root") {
				t.Errorf("expected body to contain 'Mock Root', got %q", body)
			}
		})
	}
}

func TestSPAHandler_StaticHashedAssets(t *testing.T) {
	handler := spa.NewHandlerWithFS(mockSPAFileSystem())

	tests := []struct {
		path            string
		expectedType    string
		expectedCache   string
		expectedSnippet string
	}{
		{
			path:            "/assets/index-abc1234.js",
			expectedType:    "application/javascript",
			expectedCache:   "public, max-age=31536000, immutable",
			expectedSnippet: "console.log('cloudvitta bundle');",
		},
		{
			path:            "/assets/theme-xyz5678.css",
			expectedType:    "text/css",
			expectedCache:   "public, max-age=31536000, immutable",
			expectedSnippet: "background: #0b0f17",
		},
		{
			path:            "/favicon.svg",
			expectedType:    "image/svg+xml",
			expectedCache:   "public, max-age=3600",
			expectedSnippet: "<svg xmlns=",
		},
	}

	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected status 200 OK for %s, got %d", tc.path, rec.Code)
			}

			contentType := rec.Header().Get("Content-Type")
			if !strings.Contains(contentType, tc.expectedType) {
				t.Errorf("expected Content-Type containing %q, got %q", tc.expectedType, contentType)
			}

			cacheControl := rec.Header().Get("Cache-Control")
			if cacheControl != tc.expectedCache {
				t.Errorf("expected Cache-Control %q, got %q", tc.expectedCache, cacheControl)
			}

			body := rec.Body.String()
			if !strings.Contains(body, tc.expectedSnippet) {
				t.Errorf("expected body to contain %q, got %q", tc.expectedSnippet, body)
			}
		})
	}
}

func TestSPAHandler_StrictAPIGuards(t *testing.T) {
	handler := spa.NewHandlerWithFS(mockSPAFileSystem())

	apiGuardedPaths := []string{
		"/api",
		"/api/",
		"/api/v1/prices/compute",
		"/api/v1/unknown-endpoint",
		"/mcp",
		"/mcp/",
		"/mcp/stream",
		"/docs",
		"/docs/",
		"/docs/swagger.json",
		"/healthz",
		"/readyz",
		"/metrics",
	}

	for _, path := range apiGuardedPaths {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusNotFound {
				t.Fatalf("expected 404 Not Found for guarded API path %s, got %d", path, rec.Code)
			}

			contentType := rec.Header().Get("Content-Type")
			if contentType != "application/problem+json" {
				t.Errorf("expected Content-Type 'application/problem+json', got %q", contentType)
			}

			// Ensure response is NOT HTML
			body := rec.Body.String()
			if strings.Contains(body, "<!doctype html>") || strings.Contains(body, "<html") {
				t.Fatalf("guarded API path %s returned HTML instead of RFC 7807 JSON: %s", path, body)
			}

			var problem spa.RFC7807Problem
			if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
				t.Fatalf("failed to unmarshal RFC 7807 problem json: %v", err)
			}

			if problem.Status != http.StatusNotFound {
				t.Errorf("expected status 404 in body, got %d", problem.Status)
			}
			if problem.Title != "Not Found" {
				t.Errorf("expected title 'Not Found', got %q", problem.Title)
			}
			if problem.Type != "https://cloudvitta.dev/errors/not-found" {
				t.Errorf("expected type 'https://cloudvitta.dev/errors/not-found', got %q", problem.Type)
			}
			if problem.Instance == "" {
				t.Errorf("expected instance to be non-empty")
			}
		})
	}
}

func TestSPAHandler_DisallowedMethods(t *testing.T) {
	handler := spa.NewHandlerWithFS(mockSPAFileSystem())

	disallowedMethods := []string{
		http.MethodPost,
		http.MethodPut,
		http.MethodDelete,
		http.MethodPatch,
	}

	for _, method := range disallowedMethods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/compare/compute", nil)
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusMethodNotAllowed {
				t.Fatalf("expected 405 Method Not Allowed for %s, got %d", method, rec.Code)
			}
			if rec.Header().Get("Allow") != "GET, HEAD" {
				t.Errorf("expected Allow: GET, HEAD header, got %q", rec.Header().Get("Allow"))
			}
		})
	}
}

func TestSPAHandler_DefaultConstructorEmbeddedFS(t *testing.T) {
	handler := spa.NewHandler()
	if handler == nil {
		t.Fatal("expected NewHandler() to return a valid handler")
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 from default embedded handler, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "CloudVitta") {
		t.Errorf("expected embedded index.html to contain 'CloudVitta', got %q", body)
	}
}

func TestSPAHandler_MissingIndexHTML(t *testing.T) {
	emptyFS := fstest.MapFS{}
	handler := spa.NewHandlerWithFS(emptyFS)

	req := httptest.NewRequest(http.MethodGet, "/unknown", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when index.html is missing, got %d", rec.Code)
	}
}

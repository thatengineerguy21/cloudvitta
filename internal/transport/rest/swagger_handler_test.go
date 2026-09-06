package rest_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/thatengineerguy21/CloudVitta/internal/transport/rest"
)

func TestSwaggerHandler_Endpoints(t *testing.T) {
	handler := rest.SwaggerHandler()

	t.Run("GET /docs/swagger.json returns 200 and valid OpenAPI JSON specification", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/docs/swagger.json", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200 OK, got %d", rec.Code)
		}

		contentType := rec.Header().Get("Content-Type")
		if !strings.Contains(contentType, "application/json") {
			t.Errorf("expected Content-Type containing application/json, got %q", contentType)
		}

		var spec map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &spec); err != nil {
			t.Fatalf("failed to parse JSON response: %v", err)
		}

		if swaggerVersion, ok := spec["swagger"].(string); !ok || swaggerVersion != "2.0" {
			t.Errorf("expected swagger: '2.0', got %v", spec["swagger"])
		}

		paths, ok := spec["paths"].(map[string]interface{})
		if !ok || len(paths) == 0 {
			t.Errorf("expected non-empty paths map in swagger spec")
		}
	})

	t.Run("GET /docs/doc.json returns 200 and valid JSON for backward compatibility", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/docs/doc.json", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200 OK, got %d", rec.Code)
		}

		contentType := rec.Header().Get("Content-Type")
		if !strings.Contains(contentType, "application/json") {
			t.Errorf("expected Content-Type containing application/json, got %q", contentType)
		}

		var spec map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &spec); err != nil {
			t.Fatalf("failed to parse JSON response: %v", err)
		}

		if swaggerVersion, ok := spec["swagger"].(string); !ok || swaggerVersion != "2.0" {
			t.Errorf("expected swagger: '2.0', got %v", spec["swagger"])
		}
	})

	t.Run("GET /docs/swagger.yaml returns 200 and YAML specification", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/docs/swagger.yaml", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200 OK, got %d", rec.Code)
		}

		contentType := rec.Header().Get("Content-Type")
		if !strings.Contains(contentType, "application/yaml") {
			t.Errorf("expected Content-Type containing application/yaml, got %q", contentType)
		}

		body := rec.Body.String()
		if !strings.Contains(body, "definitions:") && !strings.Contains(body, "swagger:") {
			t.Errorf("expected YAML body to contain 'definitions:' or 'swagger:'")
		}
	})

	t.Run("GET /docs/index.html serves Swagger UI interactive documentation", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/docs/index.html", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200 OK, got %d", rec.Code)
		}

		contentType := rec.Header().Get("Content-Type")
		if !strings.Contains(contentType, "text/html") {
			t.Errorf("expected Content-Type containing text/html, got %q", contentType)
		}

		body := rec.Body.String()
		if !strings.Contains(body, "swagger-ui") {
			t.Errorf("expected HTML body to contain 'swagger-ui'")
		}
	})
}

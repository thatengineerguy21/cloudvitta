package rest_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/thatengineerguy21/CloudVitta/internal/cache"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/service"
	"github.com/thatengineerguy21/CloudVitta/internal/transport/rest"
	"github.com/thatengineerguy21/CloudVitta/internal/transport/rest/middleware"
)

func TestComputeHandler_InvalidVCPU_ReturnsRFC7807(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	pricingSvc := service.NewPricingService(nil, rdb)
	handler := rest.NewComputeHandler(pricingSvc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/prices/compute?vcpu=-1", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 Bad Request", rec.Code)
	}

	contentType := rec.Header().Get("Content-Type")
	if !strings.Contains(contentType, "application/problem+json") {
		t.Errorf("Content-Type = %q, want application/problem+json", contentType)
	}

	var rfcErr middleware.RFC7807Error
	if err := json.NewDecoder(rec.Body).Decode(&rfcErr); err != nil {
		t.Fatalf("failed to decode RFC7807 JSON: %v", err)
	}

	if rfcErr.Status != 400 {
		t.Errorf("Status = %d, want 400", rfcErr.Status)
	}
	if rfcErr.Title != "Invalid query parameter" {
		t.Errorf("Title = %q, want Invalid query parameter", rfcErr.Title)
	}
	if !strings.Contains(rfcErr.Detail, "vcpu must be a positive number") {
		t.Errorf("Detail = %q, want vcpu error detail", rfcErr.Detail)
	}
	if rfcErr.Instance != "/api/v1/prices/compute?vcpu=-1" {
		t.Errorf("Instance = %q, want /api/v1/prices/compute?vcpu=-1", rfcErr.Instance)
	}
}

func TestComputeHandler_InvalidRAM_ReturnsRFC7807(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	pricingSvc := service.NewPricingService(nil, rdb)
	handler := rest.NewComputeHandler(pricingSvc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/prices/compute?ram_gb=invalid", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 Bad Request", rec.Code)
	}

	var rfcErr middleware.RFC7807Error
	if err := json.NewDecoder(rec.Body).Decode(&rfcErr); err != nil {
		t.Fatalf("failed to decode RFC7807 JSON: %v", err)
	}

	if rfcErr.Status != 400 {
		t.Errorf("Status = %d, want 400", rfcErr.Status)
	}
}

func TestComputeHandler_CurrencyWarning(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	pricingSvc := service.NewPricingService(nil, rdb)
	handler := rest.NewComputeHandler(pricingSvc)

	// Pre-warm the cache so it doesn't hit the nil DB
	cacheKey := cache.BuildKey(cache.SchemaVersion, "aws", "compute", "us-east")
	_ = cache.Warm(context.Background(), rdb, cacheKey, []domain.PriceObservation{}, cache.DefaultTTL)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/prices/compute?currency=EUR", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	// Still returns 200, but has warnings
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 OK", rec.Code)
	}

	var resp rest.ComputeComparisonResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	foundWarning := false
	for _, w := range resp.Warnings {
		if w.Code == "currency_conversion_not_yet_supported" {
			foundWarning = true
			break
		}
	}
	if !foundWarning {
		t.Errorf("expected currency conversion warning, got none")
	}

	if resp.Meta.Query["currency"] != "EUR" {
		t.Errorf("expected Meta query currency to reflect requested EUR, got %v", resp.Meta.Query["currency"])
	}
}

func TestComputeHandler_StrictFamily(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	pricingSvc := service.NewPricingService(nil, rdb)
	handler := rest.NewComputeHandler(pricingSvc)

	cacheKey := cache.BuildKey(cache.SchemaVersion, "aws", "compute", "us-east")
	_ = cache.Warm(context.Background(), rdb, cacheKey, []domain.PriceObservation{}, cache.DefaultTTL)

	// Since PricingService uses DB or Cache, we can test it just parses correctly.
	// But without a DB, getComputePrices will return no rows or error if DB isn't mocked.
	// In the handler, it just processes what comes out. We already tested the strict_family matching in match_test.go.
	// We'll just ensure a request with strict_family=true doesn't error out (it parses the boolean correctly).
	req := httptest.NewRequest(http.MethodGet, "/api/v1/prices/compute?strict_family=true", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 OK", rec.Code)
	}
}

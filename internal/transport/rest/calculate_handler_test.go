package rest_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
	"github.com/thatengineerguy21/CloudVitta/internal/cache"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/service"
	"github.com/thatengineerguy21/CloudVitta/internal/store"
	"github.com/thatengineerguy21/CloudVitta/internal/transport/rest"
	"github.com/thatengineerguy21/CloudVitta/internal/transport/rest/middleware"
)

func TestCalculateHandler_InvalidMethod_ReturnsRFC7807(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	pricingSvc := service.NewPricingService(nil, rdb)
	handler := rest.NewCalculateHandler(pricingSvc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/calculate", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405 Method Not Allowed", rec.Code)
	}
}

func TestCalculateHandler_NoCategories_ReturnsRFC7807(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	pricingSvc := service.NewPricingService(nil, rdb)
	handler := rest.NewCalculateHandler(pricingSvc)

	body := []byte(`{"region": "us-east"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/calculate", bytes.NewBuffer(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 Bad Request", rec.Code)
	}

	var rfcErr middleware.RFC7807Error
	if err := json.NewDecoder(rec.Body).Decode(&rfcErr); err != nil {
		t.Fatalf("failed to decode RFC7807 JSON: %v", err)
	}

	if !strings.Contains(rfcErr.Detail, "At least one of 'compute', 'storage', or 'network' must be specified.") {
		t.Errorf("Unexpected error detail: %s", rfcErr.Detail)
	}
}

func TestCalculateHandler_HappyPath(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	ctx := context.Background()

	// Add compute and storage for AWS
	awsComputeObs := []domain.PriceObservation{
		{
			Provider:        "aws",
			ServiceCategory: "compute",
			SkuID:           "SKU-AWS-C1",
			PriceAmount:     decimal.RequireFromString("0.10"),
			Unit:            "hour",
			Attributes:      domain.ComputeAttributes{VCPU: 2, RAMGB: 4, Family: "general"},
			FetchedAt:       time.Now().UTC(),
		},
	}
	awsStorageObs := []domain.PriceObservation{
		{
			Provider:          "aws",
			ServiceCategory:   "storage",
			SkuID:             "SKU-AWS-S1",
			PriceAmount:       decimal.RequireFromString("0.02"), // $0.02/GB-Mo
			Unit:              "GB-Mo",
			StorageAttributes: domain.StorageAttributes{SizeGB: 100, StorageClass: "standard"},
			FetchedAt:         time.Now().UTC(),
		},
	}

	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "aws", "compute", "us-east"), awsComputeObs, cache.DefaultTTL)
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "aws", "storage", "us-east"), awsStorageObs, cache.DefaultTTL)

	// Add only compute for Azure
	azureComputeObs := []domain.PriceObservation{
		{
			Provider:        "azure",
			ServiceCategory: "compute",
			SkuID:           "SKU-AZ-C1",
			PriceAmount:     decimal.RequireFromString("0.15"),
			Unit:            "hour",
			Attributes:      domain.ComputeAttributes{VCPU: 2, RAMGB: 4, Family: "general"},
			FetchedAt:       time.Now().UTC(),
		},
	}
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "azure", "compute", "us-east"), azureComputeObs, cache.DefaultTTL)
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "azure", "storage", "us-east"), []domain.PriceObservation{}, cache.DefaultTTL)

	// GCP gets nothing
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "gcp", "compute", "us-east"), []domain.PriceObservation{}, cache.DefaultTTL)
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "gcp", "storage", "us-east"), []domain.PriceObservation{}, cache.DefaultTTL)

	pricingSvc := service.NewPricingService(nil, rdb)
	handler := rest.NewCalculateHandler(pricingSvc)

	body := []byte(`{
		"region": "us-east",
		"compute": {
			"vcpu": 2,
			"ram_gb": 4,
			"family": "general"
		},
		"storage": {
			"size_gb": 100,
			"storage_class": "standard"
		}
	}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/calculate", bytes.NewBuffer(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 OK. Body: %s", rec.Code, rec.Body.String())
	}

	var resp rest.CalculateResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Results) != 2 {
		t.Fatalf("got %d results, want 2 (AWS, Azure)", len(resp.Results))
	}

	var awsResult, azureResult *rest.CalculateProviderResult
	for i := range resp.Results {
		if resp.Results[i].Provider == "aws" {
			awsResult = &resp.Results[i]
		}
		if resp.Results[i].Provider == "azure" {
			azureResult = &resp.Results[i]
		}
	}

	if awsResult == nil || azureResult == nil {
		t.Fatalf("missing expected providers in result")
	}

	// AWS should be full match
	if awsResult.Partial {
		t.Errorf("expected AWS to not be partial")
	}
	if awsResult.TotalNormalizedHourlyUSD == nil {
		t.Errorf("expected AWS to have total")
	}

	// Storage cost = 0.02 * 100 = 2/mo. Hourly = 2/730 = 0.00273972602...
	// Total = 0.10 + 2/730

	// Azure should be partial match
	if !azureResult.Partial {
		t.Errorf("expected Azure to be partial")
	}
	if azureResult.TotalNormalizedHourlyUSD != nil {
		t.Errorf("expected Azure to NOT have total")
	}
	if azureResult.PartialTotalNormalizedHourlyUSD == nil {
		t.Errorf("expected Azure to have partial total")
	}
}

func TestCalculateHandler_AllProvidersFail_Returns500(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	failingQueries := store.New(&failingDBTX{})
	pricingSvc := service.NewPricingService(failingQueries, rdb)
	handler := rest.NewCalculateHandler(pricingSvc)

	body := []byte(`{"compute": {"vcpu": 2, "ram_gb": 4}}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/calculate", bytes.NewBuffer(body))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 Internal Server Error", rec.Code)
	}

	var rfcErr middleware.RFC7807Error
	if err := json.NewDecoder(rec.Body).Decode(&rfcErr); err != nil {
		t.Fatalf("failed to decode RFC7807 JSON: %v", err)
	}
	if rfcErr.Status != 500 {
		t.Errorf("Status = %d, want 500", rfcErr.Status)
	}
}

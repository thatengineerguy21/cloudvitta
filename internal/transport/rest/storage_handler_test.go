package rest_test

import (
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
	"github.com/thatengineerguy21/CloudVitta/internal/transport/rest"
	"github.com/thatengineerguy21/CloudVitta/internal/transport/rest/middleware"
)

func TestStorageHandler_InvalidSizeGB_ReturnsRFC7807(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	pricingSvc := service.NewPricingService(nil, rdb)
	handler := rest.NewStorageHandler(pricingSvc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/prices/storage?size_gb=-10", nil)
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
	if !strings.Contains(rfcErr.Detail, "size_gb must be a positive number") {
		t.Errorf("Detail = %q, want size_gb error detail", rfcErr.Detail)
	}
}

func TestStorageHandler_MethodNotAllowed(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	pricingSvc := service.NewPricingService(nil, rdb)
	handler := rest.NewStorageHandler(pricingSvc)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/prices/storage", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405 Method Not Allowed", rec.Code)
	}
}

func TestStorageHandler_HappyPath_CalculatesMonthlyCosts(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	// Seed cache for AWS, Azure, and GCP storage
	ctx := context.Background()
	awsObs := []domain.PriceObservation{
		{
			Provider:          "aws",
			ServiceCategory:   "storage",
			SkuID:             "SKU-AWS-S3-STD",
			DisplayName:       "S3 Standard",
			Region:            "us-east-1",
			RegionGroup:       "us-east",
			Unit:              "GB-Mo",
			PriceAmount:       decimal.RequireFromString("0.023"),
			PriceCurrency:     "USD",
			PricingModel:      "OnDemand",
			StorageAttributes: domain.StorageAttributes{SizeGB: 1, StorageClass: "standard"},
			FetchedAt:         time.Now().UTC(),
		},
	}
	azureObs := []domain.PriceObservation{
		{
			Provider:          "azure",
			ServiceCategory:   "storage",
			SkuID:             "SKU-AZURE-BLOB-HOT",
			DisplayName:       "Blob Storage Hot",
			Region:            "eastus",
			RegionGroup:       "us-east",
			Unit:              "GB-Mo",
			PriceAmount:       decimal.RequireFromString("0.0184"),
			PriceCurrency:     "USD",
			PricingModel:      "OnDemand",
			StorageAttributes: domain.StorageAttributes{SizeGB: 1, StorageClass: "standard"},
			FetchedAt:         time.Now().UTC(),
		},
	}
	gcpObs := []domain.PriceObservation{
		{
			Provider:          "gcp",
			ServiceCategory:   "storage",
			SkuID:             "SKU-GCP-GCS-STD",
			DisplayName:       "Standard Storage",
			Region:            "us-east1",
			RegionGroup:       "us-east",
			Unit:              "GB-Mo",
			PriceAmount:       decimal.RequireFromString("0.020"),
			PriceCurrency:     "USD",
			PricingModel:      "OnDemand",
			StorageAttributes: domain.StorageAttributes{SizeGB: 1, StorageClass: "standard"},
			FetchedAt:         time.Now().UTC(),
		},
	}

	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "aws", "storage", "us-east"), awsObs, cache.DefaultTTL)
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "azure", "storage", "us-east"), azureObs, cache.DefaultTTL)
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "gcp", "storage", "us-east"), gcpObs, cache.DefaultTTL)

	pricingSvc := service.NewPricingService(nil, rdb)
	handler := rest.NewStorageHandler(pricingSvc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/prices/storage?size_gb=500&region=us-east&storage_class=standard", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 OK. Body: %s", rec.Code, rec.Body.String())
	}

	var resp rest.StorageComparisonResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Meta.APIVersion != "v1" {
		t.Errorf("APIVersion = %q, want v1", resp.Meta.APIVersion)
	}
	if len(resp.Results) != 3 {
		t.Fatalf("got %d results, want 3 (AWS, Azure, GCP)", len(resp.Results))
	}

	// Verify AWS 500 GB cost = 500 * 0.023 = 11.5
	var awsResult *rest.StorageResultEntry
	for i := range resp.Results {
		if resp.Results[i].Provider == "aws" {
			awsResult = &resp.Results[i]
			break
		}
	}
	if awsResult == nil {
		t.Fatalf("missing aws result")
	}
	if awsResult.MonthlyCostUSD.String() != "11.5" {
		t.Errorf("AWS MonthlyCostUSD = %s, want 11.5", awsResult.MonthlyCostUSD.String())
	}
	if awsResult.MatchQuality != "exact" {
		t.Errorf("AWS MatchQuality = %q, want exact", awsResult.MatchQuality)
	}

	// Verify Azure 500 GB cost = 500 * 0.0184 = 9.2
	var azResult *rest.StorageResultEntry
	for i := range resp.Results {
		if resp.Results[i].Provider == "azure" {
			azResult = &resp.Results[i]
			break
		}
	}
	if azResult == nil {
		t.Fatalf("missing azure result")
	}
	if azResult.MonthlyCostUSD.String() != "9.2" {
		t.Errorf("Azure MonthlyCostUSD = %s, want 9.2", azResult.MonthlyCostUSD.String())
	}

	// Verify GCP 500 GB cost = 500 * 0.02 = 10
	var gcpResult *rest.StorageResultEntry
	for i := range resp.Results {
		if resp.Results[i].Provider == "gcp" {
			gcpResult = &resp.Results[i]
			break
		}
	}
	if gcpResult == nil {
		t.Fatalf("missing gcp result")
	}
	if gcpResult.MonthlyCostUSD.String() != "10" {
		t.Errorf("GCP MonthlyCostUSD = %s, want 10", gcpResult.MonthlyCostUSD.String())
	}
}

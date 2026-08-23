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

func TestDatabaseNoSQLHandler_InvalidParameters_ReturnsRFC7807(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	pricingSvc := service.NewPricingService(nil, rdb)
	handler := rest.NewDatabaseNoSQLHandler(pricingSvc)

	tests := []struct {
		name       string
		query      string
		wantDetail string
	}{
		{
			name:       "negative read_units",
			query:      "read_units=-10",
			wantDetail: "read_units must be a non-negative number",
		},
		{
			name:       "exceeds max read_units",
			query:      "read_units=10000001",
			wantDetail: "read_units must be a non-negative number up to 10000000",
		},
		{
			name:       "negative write_units",
			query:      "write_units=-10",
			wantDetail: "write_units must be a non-negative number",
		},
		{
			name:       "exceeds max write_units",
			query:      "write_units=10000001",
			wantDetail: "write_units must be a non-negative number up to 10000000",
		},
		{
			name:       "negative storage_gb",
			query:      "storage_gb=-100",
			wantDetail: "storage_gb must be a non-negative number up to 1000000",
		},
		{
			name:       "exceeds max storage_gb",
			query:      "storage_gb=1000001",
			wantDetail: "storage_gb must be a non-negative number up to 1000000",
		},
		{
			name:       "unmapped data model",
			query:      "data_model=unknown_model_type",
			wantDetail: "data_model must be a supported NoSQL data model",
		},
		{
			name:       "invalid pricing mode",
			query:      "pricing_mode=unsupported_mode",
			wantDetail: "pricing_mode must be provisioned, on_demand, or serverless",
		},
		{
			name:       "ondemand pricing mode rejected",
			query:      "pricing_mode=ondemand",
			wantDetail: "pricing_mode must be provisioned, on_demand, or serverless",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/prices/database-nosql?"+tt.query, nil)
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
			if !strings.Contains(rfcErr.Detail, tt.wantDetail) {
				t.Errorf("Detail = %q, want substring %q", rfcErr.Detail, tt.wantDetail)
			}
		})
	}
}

func TestDatabaseNoSQLHandler_MethodNotAllowed_Returns405(t *testing.T) {
	pricingSvc := service.NewPricingService(nil, nil)
	handler := rest.NewDatabaseNoSQLHandler(pricingSvc)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/prices/database-nosql", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405 Method Not Allowed", rec.Code)
	}
}

func TestDatabaseNoSQLHandler_Success(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	now := time.Now().UTC()

	// Seed cache with AWS DynamoDB observations
	awsObs := []domain.PriceObservation{
		{
			Provider:        "aws",
			ServiceCategory: "database_nosql",
			SkuID:           "SKU-AWS-DDB-READ-PROV",
			Region:          "us-east-1",
			RegionGroup:     "us-east",
			PriceAmount:     decimal.NewFromFloat(0.00013),
			PriceCurrency:   "USD",
			Unit:            "Hrs",
			DatabaseNoSQLAttributes: domain.DatabaseNoSQLAttributes{
				DataModel:     "document",
				PricingMode:   "provisioned",
				ReadUnits:     1,
				ComponentType: "throughput",
			},
			FetchedAt: now,
		},
		{
			Provider:        "aws",
			ServiceCategory: "database_nosql",
			SkuID:           "SKU-AWS-DDB-WRITE-PROV",
			Region:          "us-east-1",
			RegionGroup:     "us-east",
			PriceAmount:     decimal.NewFromFloat(0.00065),
			PriceCurrency:   "USD",
			Unit:            "Hrs",
			DatabaseNoSQLAttributes: domain.DatabaseNoSQLAttributes{
				DataModel:     "document",
				PricingMode:   "provisioned",
				WriteUnits:    1,
				ComponentType: "throughput",
			},
			FetchedAt: now,
		},
		{
			Provider:        "aws",
			ServiceCategory: "database_nosql",
			SkuID:           "SKU-AWS-DDB-STORAGE",
			Region:          "us-east-1",
			RegionGroup:     "us-east",
			PriceAmount:     decimal.NewFromFloat(0.25),
			PriceCurrency:   "USD",
			Unit:            "GB-Mo",
			DatabaseNoSQLAttributes: domain.DatabaseNoSQLAttributes{
				DataModel:     "document",
				PricingMode:   "provisioned",
				StorageGB:     1,
				StorageClass:  "standard",
				ComponentType: "storage",
			},
			FetchedAt: now,
		},
	}
	awsKey := cache.BuildKey(cache.SchemaVersion, "aws", "database_nosql", "us-east-1")
	if err := cache.Warm(context.Background(), rdb, awsKey, awsObs, time.Hour); err != nil {
		t.Fatalf("failed to warm aws cache: %v", err)
	}

	// Seed cache with Azure Cosmos DB observations
	azureObs := []domain.PriceObservation{
		{
			Provider:        "azure",
			ServiceCategory: "database_nosql",
			SkuID:           "SKU-AZURE-COSMOS-100RU",
			Region:          "eastus",
			RegionGroup:     "us-east",
			PriceAmount:     decimal.NewFromFloat(0.008),
			PriceCurrency:   "USD",
			Unit:            "Hrs",
			DatabaseNoSQLAttributes: domain.DatabaseNoSQLAttributes{
				DataModel:     "document",
				PricingMode:   "provisioned",
				ReadUnits:     100,
				WriteUnits:    20,
				ComponentType: "throughput",
			},
			FetchedAt: now,
		},
		{
			Provider:        "azure",
			ServiceCategory: "database_nosql",
			SkuID:           "SKU-AZURE-COSMOS-STORAGE",
			Region:          "eastus",
			RegionGroup:     "us-east",
			PriceAmount:     decimal.NewFromFloat(0.25),
			PriceCurrency:   "USD",
			Unit:            "GB-Mo",
			DatabaseNoSQLAttributes: domain.DatabaseNoSQLAttributes{
				DataModel:     "document",
				PricingMode:   "provisioned",
				StorageGB:     1,
				StorageClass:  "standard",
				ComponentType: "storage",
			},
			FetchedAt: now,
		},
	}
	azureKey := cache.BuildKey(cache.SchemaVersion, "azure", "database_nosql", "eastus")
	if err := cache.Warm(context.Background(), rdb, azureKey, azureObs, time.Hour); err != nil {
		t.Fatalf("failed to warm azure cache: %v", err)
	}

	pricingSvc := service.NewPricingService(nil, rdb)
	handler := rest.NewDatabaseNoSQLHandler(pricingSvc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/prices/database-nosql?read_units=100&write_units=20&storage_gb=50&pricing_mode=provisioned", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 OK: body = %s", rec.Code, rec.Body.String())
	}

	var resp rest.DatabaseNoSQLComparisonResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response JSON: %v", err)
	}

	if resp.Meta.APIVersion != "v1" {
		t.Errorf("expected APIVersion v1, got %s", resp.Meta.APIVersion)
	}

	if len(resp.Results) < 2 {
		t.Fatalf("expected at least 2 results (AWS, Azure), got %d", len(resp.Results))
	}

	for _, res := range resp.Results {
		if res.MatchQuality == "" || res.MatchQuality == "none" {
			t.Errorf("unexpected match quality for %s: %s", res.Provider, res.MatchQuality)
		}
		if res.Price.Currency != "USD" {
			t.Errorf("expected price currency USD, got %s", res.Price.Currency)
		}
		if res.NormalizedHourlyUSD.LessThanOrEqual(decimal.Zero) {
			t.Errorf("expected positive normalized hourly USD for %s, got %s", res.Provider, res.NormalizedHourlyUSD)
		}
	}
}

func TestDatabaseNoSQLHandler_CurrencyWarning(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	now := time.Now().UTC()
	awsObs := []domain.PriceObservation{
		{
			Provider:        "aws",
			ServiceCategory: "database_nosql",
			SkuID:           "SKU-AWS-DDB-READ-PROV",
			Region:          "us-east-1",
			RegionGroup:     "us-east",
			PriceAmount:     decimal.NewFromFloat(0.00013),
			PriceCurrency:   "USD",
			Unit:            "Hrs",
			DatabaseNoSQLAttributes: domain.DatabaseNoSQLAttributes{
				DataModel:     "document",
				PricingMode:   "provisioned",
				ReadUnits:     1,
				ComponentType: "throughput",
			},
			FetchedAt: now,
		},
		{
			Provider:        "aws",
			ServiceCategory: "database_nosql",
			SkuID:           "SKU-AWS-DDB-STORAGE",
			Region:          "us-east-1",
			RegionGroup:     "us-east",
			PriceAmount:     decimal.NewFromFloat(0.25),
			PriceCurrency:   "USD",
			Unit:            "GB-Mo",
			DatabaseNoSQLAttributes: domain.DatabaseNoSQLAttributes{
				DataModel:     "document",
				PricingMode:   "provisioned",
				StorageGB:     1,
				StorageClass:  "standard",
				ComponentType: "storage",
			},
			FetchedAt: now,
		},
	}
	awsKey := cache.BuildKey(cache.SchemaVersion, "aws", "database_nosql", "us-east-1")
	_ = cache.Warm(context.Background(), rdb, awsKey, awsObs, time.Hour)

	pricingSvc := service.NewPricingService(nil, rdb)
	handler := rest.NewDatabaseNoSQLHandler(pricingSvc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/prices/database-nosql?currency=EUR", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 OK: body = %s", rec.Code, rec.Body.String())
	}

	var resp rest.DatabaseNoSQLComparisonResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response JSON: %v", err)
	}

	var hasCurrencyWarning bool
	for _, w := range resp.Warnings {
		if w.Code == "currency_conversion_not_yet_supported" {
			hasCurrencyWarning = true
			break
		}
	}
	if !hasCurrencyWarning {
		t.Errorf("expected currency_conversion_not_yet_supported warning when requesting EUR")
	}
}

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

func TestDatabaseHandler_InvalidParameters_ReturnsRFC7807(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	pricingSvc := service.NewPricingService(nil, rdb)
	handler := rest.NewDatabaseHandler(pricingSvc)

	tests := []struct {
		name       string
		query      string
		wantDetail string
	}{
		{
			name:       "negative vcpu",
			query:      "vcpu=-4",
			wantDetail: "vcpu must be a positive number",
		},
		{
			name:       "invalid vcpu text",
			query:      "vcpu=abc",
			wantDetail: "vcpu must be a positive number",
		},
		{
			name:       "negative ram_gb",
			query:      "ram_gb=-16",
			wantDetail: "ram_gb must be a positive number",
		},
		{
			name:       "negative storage_gb",
			query:      "storage_gb=-100",
			wantDetail: "storage_gb must be a positive number up to 1000000",
		},
		{
			name:       "exceeds max storage_gb",
			query:      "storage_gb=1000001",
			wantDetail: "storage_gb must be a positive number up to 1000000",
		},
		{
			name:       "negative iops",
			query:      "iops=-100",
			wantDetail: "iops must be a positive integer",
		},
		{
			name:       "unmapped database engine",
			query:      "engine=foobar_db",
			wantDetail: "engine must be a supported database engine",
		},
		{
			name:       "gated mariadb engine (stage 3.1)",
			query:      "engine=mariadb",
			wantDetail: "engine must be a supported database engine",
		},
		{
			name:       "gated oracle engine (stage 3.1)",
			query:      "engine=oracle",
			wantDetail: "engine must be a supported database engine",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/prices/database?"+tt.query, nil)
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

func TestDatabaseHandler_MethodNotAllowed_Returns405(t *testing.T) {
	pricingSvc := service.NewPricingService(nil, nil)
	handler := rest.NewDatabaseHandler(pricingSvc)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/prices/database", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405 Method Not Allowed", rec.Code)
	}
}

func TestDatabaseHandler_Success(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	now := time.Now().UTC()

	// Seed cache with AWS database observations (instance + storage)
	awsObs := []domain.PriceObservation{
		{
			Provider:        "aws",
			ServiceCategory: "database_rdbms",
			SkuID:           "AWS-RDS-PG-M6G-XLARGE",
			Region:          "us-east-1",
			RegionGroup:     "us-east",
			PriceAmount:     decimal.NewFromFloat(0.26),
			PriceCurrency:   "USD",
			Unit:            "Hrs",
			DatabaseRDBMSAttributes: domain.DatabaseRDBMSAttributes{
				Engine:         "postgresql",
				VCPU:           4,
				RAMGB:          16,
				MultiAZ:        false,
				DeploymentTier: "standard",
				ComponentType:  "instance",
			},
			FetchedAt: now,
		},
		{
			Provider:        "aws",
			ServiceCategory: "database_rdbms",
			SkuID:           "AWS-RDS-STORAGE-GP3",
			Region:          "us-east-1",
			RegionGroup:     "us-east",
			PriceAmount:     decimal.NewFromFloat(0.115),
			PriceCurrency:   "USD",
			Unit:            "GB-Mo",
			DatabaseRDBMSAttributes: domain.DatabaseRDBMSAttributes{
				Engine:        "any",
				StorageGB:     1,
				MultiAZ:       false,
				StorageFamily: "gp3",
				ComponentType: "storage",
			},
			FetchedAt: now,
		},
	}

	awsKey := cache.BuildKey(cache.SchemaVersion, "aws", "database_rdbms", "us-east-1")
	if err := cache.Warm(context.Background(), rdb, awsKey, awsObs, 10*time.Minute); err != nil {
		t.Fatalf("failed to warm aws cache: %v", err)
	}

	// Seed Azure cache with database observations
	azureObs := []domain.PriceObservation{
		{
			Provider:        "azure",
			ServiceCategory: "database_rdbms",
			SkuID:           "AZURE-PG-GP-4VCORE",
			Region:          "eastus",
			RegionGroup:     "us-east",
			PriceAmount:     decimal.NewFromFloat(0.28),
			PriceCurrency:   "USD",
			Unit:            "Hrs",
			DatabaseRDBMSAttributes: domain.DatabaseRDBMSAttributes{
				Engine:         "postgresql",
				VCPU:           4,
				RAMGB:          16,
				MultiAZ:        false,
				DeploymentTier: "flexible",
				ComponentType:  "instance",
			},
			FetchedAt: now,
		},
		{
			Provider:        "azure",
			ServiceCategory: "database_rdbms",
			SkuID:           "AZURE-PG-STORAGE",
			Region:          "eastus",
			RegionGroup:     "us-east",
			PriceAmount:     decimal.NewFromFloat(0.115),
			PriceCurrency:   "USD",
			Unit:            "GB-Mo",
			DatabaseRDBMSAttributes: domain.DatabaseRDBMSAttributes{
				Engine:        "postgresql",
				StorageGB:     1,
				MultiAZ:       false,
				StorageFamily: "ssd",
				ComponentType: "storage",
			},
			FetchedAt: now,
		},
	}
	azureKey := cache.BuildKey(cache.SchemaVersion, "azure", "database_rdbms", "eastus")
	if err := cache.Warm(context.Background(), rdb, azureKey, azureObs, 10*time.Minute); err != nil {
		t.Fatalf("failed to warm azure cache: %v", err)
	}

	pricingSvc := service.NewPricingService(nil, rdb)
	handler := rest.NewDatabaseHandler(pricingSvc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/prices/database?engine=postgresql&vcpu=4&ram_gb=16&storage_gb=100&region=us-east&currency=EUR", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 OK, body: %s", rec.Code, rec.Body.String())
	}

	var resp rest.DatabaseComparisonResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response JSON: %v", err)
	}

	if len(resp.Results) != 2 {
		t.Fatalf("expected 2 results (aws and azure), got %d", len(resp.Results))
	}

	if resp.Meta.Query["engine"] != "postgresql" {
		t.Errorf("expected query meta engine postgresql, got %v", resp.Meta.Query["engine"])
	}

	// Verify currency warning
	var hasCurrWarning bool
	for _, w := range resp.Warnings {
		if w.Code == "non_usd_currency_unsupported" {
			hasCurrWarning = true
		}
	}
	if !hasCurrWarning {
		t.Errorf("expected non_usd_currency_unsupported warning when requesting EUR")
	}

	// Verify results
	for _, res := range resp.Results {
		if res.MatchQuality != "exact" {
			t.Errorf("expected match_quality exact for %s, got %s", res.Provider, res.MatchQuality)
		}
		if res.MatchedSpec.Engine != "postgresql" {
			t.Errorf("expected matched engine postgresql, got %s", res.MatchedSpec.Engine)
		}
		if res.MatchedSpec.StorageGB != 100 {
			t.Errorf("expected matched storage 100, got %f", res.MatchedSpec.StorageGB)
		}
		if res.Price.Currency != "USD" {
			t.Errorf("expected currency USD, got %s", res.Price.Currency)
		}
	}
}

// TestDatabaseHandler_EngineMismatchWarning verifies that candidates present with mismatched engine
// yield warnings[].code: "engine_mismatch_excluded" rather than generic "no_match" (Item 3).
func TestDatabaseHandler_EngineMismatchWarning(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	now := time.Now().UTC()

	// Seed cache with AWS database observations (PostgreSQL only)
	awsObs := []domain.PriceObservation{
		{
			Provider:        "aws",
			ServiceCategory: "database_rdbms",
			SkuID:           "AWS-RDS-PG-M6G-XLARGE",
			Region:          "us-east-1",
			RegionGroup:     "us-east",
			PriceAmount:     decimal.NewFromFloat(0.26),
			PriceCurrency:   "USD",
			Unit:            "Hrs",
			DatabaseRDBMSAttributes: domain.DatabaseRDBMSAttributes{
				Engine:         "postgresql",
				VCPU:           4,
				RAMGB:          16,
				MultiAZ:        false,
				DeploymentTier: "standard",
				ComponentType:  "instance",
			},
			FetchedAt: now,
		},
	}

	awsKey := cache.BuildKey(cache.SchemaVersion, "aws", "database_rdbms", "us-east-1")
	if err := cache.Warm(context.Background(), rdb, awsKey, awsObs, 10*time.Minute); err != nil {
		t.Fatalf("failed to warm aws cache: %v", err)
	}

	pricingSvc := service.NewPricingService(nil, rdb)
	handler := rest.NewDatabaseHandler(pricingSvc)

	// Request MySQL when only PostgreSQL candidate is in cache
	req := httptest.NewRequest(http.MethodGet, "/api/v1/prices/database?engine=mysql&vcpu=4&ram_gb=16&storage_gb=100&region=us-east", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 OK, body: %s", rec.Code, rec.Body.String())
	}

	var resp rest.DatabaseComparisonResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response JSON: %v", err)
	}

	if len(resp.Results) != 0 {
		t.Errorf("expected 0 results due to engine mismatch, got %d", len(resp.Results))
	}

	var hasMismatchWarning bool
	var hasBareNoMatch bool
	for _, w := range resp.Warnings {
		if w.Provider == "aws" {
			if w.Code == "engine_mismatch_excluded" {
				hasMismatchWarning = true
			}
			if w.Code == "no_match" {
				hasBareNoMatch = true
			}
		}
	}

	if !hasMismatchWarning {
		t.Errorf("expected warnings[].code 'engine_mismatch_excluded' for aws, got warnings: %+v", resp.Warnings)
	}
	if hasBareNoMatch {
		t.Errorf("expected no bare 'no_match' warning when candidate was excluded for engine mismatch, got warnings: %+v", resp.Warnings)
	}
}

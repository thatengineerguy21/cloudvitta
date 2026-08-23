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

func TestServerlessHandler_InvalidParameters_ReturnsRFC7807(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	pricingSvc := service.NewPricingService(nil, rdb)
	handler := rest.NewServerlessHandler(pricingSvc)

	tests := []struct {
		name       string
		query      string
		wantDetail string
	}{
		{
			name:       "invalid architecture",
			query:      "architecture=mips",
			wantDetail: "architecture must be a supported CPU architecture",
		},
		{
			name:       "invalid requests_per_month negative",
			query:      "requests_per_month=-10",
			wantDetail: "requests_per_month must be a non-negative number",
		},
		{
			name:       "memory_mb too low",
			query:      "memory_mb=64",
			wantDetail: "memory_mb must be between 128 and 10240 MB",
		},
		{
			name:       "memory_mb too high",
			query:      "memory_mb=20480",
			wantDetail: "memory_mb must be between 128 and 10240 MB",
		},
		{
			name:       "execution_duration_ms too low",
			query:      "execution_duration_ms=0",
			wantDetail: "execution_duration_ms must be between 1 and 900000 ms",
		},
		{
			name:       "execution_duration_ms too high",
			query:      "execution_duration_ms=1000000",
			wantDetail: "execution_duration_ms must be between 1 and 900000 ms",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/prices/serverless?"+tt.query, nil)
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

			if !strings.Contains(rfcErr.Detail, tt.wantDetail) {
				t.Errorf("RFC7807 Detail = %q, want substring %q", rfcErr.Detail, tt.wantDetail)
			}
		})
	}
}

func TestServerlessHandler_MethodNotAllowed(t *testing.T) {
	pricingSvc := service.NewPricingService(nil, nil)
	handler := rest.NewServerlessHandler(pricingSvc)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/prices/serverless", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405 Method Not Allowed", rec.Code)
	}
}

func TestServerlessHandler_Success_Comparison(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	ctx := context.Background()
	fixedTime := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)
	regionGroup := "us-east"

	awsObs := []domain.PriceObservation{
		{
			Provider:        "aws",
			ServiceCategory: "serverless",
			SkuID:           "SKU-AWS-REQ-X86",
			Region:          "us-east-1",
			RegionGroup:     regionGroup,
			PriceAmount:     decimal.RequireFromString("0.20"),
			Unit:            "Requests",
			PriceCurrency:   "USD",
			ServerlessRateAttributes: domain.ServerlessRateAttributes{
				Architecture:  domain.ArchitectureX86_64,
				Tier:          domain.ServerlessTierConsumption,
				ComponentType: domain.ComponentTypeRequestFee,
				Unit:          domain.UnitPerMillionRequests,
			},
			FetchedAt: fixedTime,
		},
		{
			Provider:        "aws",
			ServiceCategory: "serverless",
			SkuID:           "SKU-AWS-DUR-X86",
			Region:          "us-east-1",
			RegionGroup:     regionGroup,
			PriceAmount:     decimal.RequireFromString("0.0000166667"),
			Unit:            "Seconds",
			PriceCurrency:   "USD",
			ServerlessRateAttributes: domain.ServerlessRateAttributes{
				Architecture:  domain.ArchitectureX86_64,
				Tier:          domain.ServerlessTierConsumption,
				ComponentType: domain.ComponentTypeDurationFee,
				Unit:          domain.UnitPerGBSecond,
			},
			FetchedAt: fixedTime,
		},
		{
			Provider:        "aws",
			ServiceCategory: "serverless",
			SkuID:           "SKU-AWS-REQ-ARM",
			Region:          "us-east-1",
			RegionGroup:     regionGroup,
			PriceAmount:     decimal.RequireFromString("0.20"),
			Unit:            "Requests",
			PriceCurrency:   "USD",
			ServerlessRateAttributes: domain.ServerlessRateAttributes{
				Architecture:  domain.ArchitectureARM64,
				Tier:          domain.ServerlessTierConsumption,
				ComponentType: domain.ComponentTypeRequestFee,
				Unit:          domain.UnitPerMillionRequests,
			},
			FetchedAt: fixedTime,
		},
		{
			Provider:        "aws",
			ServiceCategory: "serverless",
			SkuID:           "SKU-AWS-DUR-ARM",
			Region:          "us-east-1",
			RegionGroup:     regionGroup,
			PriceAmount:     decimal.RequireFromString("0.0000133334"),
			Unit:            "Seconds",
			PriceCurrency:   "USD",
			ServerlessRateAttributes: domain.ServerlessRateAttributes{
				Architecture:  domain.ArchitectureARM64,
				Tier:          domain.ServerlessTierConsumption,
				ComponentType: domain.ComponentTypeDurationFee,
				Unit:          domain.UnitPerGBSecond,
			},
			FetchedAt: fixedTime,
		},
	}
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "aws", "serverless", "us-east-1"), awsObs, cache.DefaultTTL)

	azureObs := []domain.PriceObservation{
		{
			Provider:        "azure",
			ServiceCategory: "serverless",
			SkuID:           "SKU-AZURE-REQ",
			Region:          "eastus",
			RegionGroup:     regionGroup,
			PriceAmount:     decimal.RequireFromString("0.000002"),
			Unit:            "10",
			PriceCurrency:   "USD",
			ServerlessRateAttributes: domain.ServerlessRateAttributes{
				Architecture:  domain.ArchitectureX86_64,
				Tier:          domain.ServerlessTierConsumption,
				ComponentType: domain.ComponentTypeRequestFee,
				Unit:          domain.UnitPer10Requests,
			},
			FetchedAt: fixedTime,
		},
		{
			Provider:        "azure",
			ServiceCategory: "serverless",
			SkuID:           "SKU-AZURE-DUR",
			Region:          "eastus",
			RegionGroup:     regionGroup,
			PriceAmount:     decimal.RequireFromString("0.000016"),
			Unit:            "1 GB Second",
			PriceCurrency:   "USD",
			ServerlessRateAttributes: domain.ServerlessRateAttributes{
				Architecture:  domain.ArchitectureX86_64,
				Tier:          domain.ServerlessTierConsumption,
				ComponentType: domain.ComponentTypeDurationFee,
				Unit:          domain.UnitPerGBSecond,
			},
			FetchedAt: fixedTime,
		},
	}
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "azure", "serverless", "eastus"), azureObs, cache.DefaultTTL)

	gcpObs := []domain.PriceObservation{
		{
			Provider:        "gcp",
			ServiceCategory: "serverless",
			SkuID:           "SKU-GCP-REQ",
			Region:          "us-east4",
			RegionGroup:     regionGroup,
			PriceAmount:     decimal.RequireFromString("0.0000004"),
			Unit:            "Calls",
			PriceCurrency:   "USD",
			ServerlessRateAttributes: domain.ServerlessRateAttributes{
				Architecture:  domain.ArchitectureX86_64,
				Tier:          domain.ServerlessTierConsumption,
				ComponentType: domain.ComponentTypeRequestFee,
				Unit:          domain.UnitPerRequest,
			},
			FetchedAt: fixedTime,
		},
		{
			Provider:        "gcp",
			ServiceCategory: "serverless",
			SkuID:           "SKU-GCP-CPU",
			Region:          "us-east4",
			RegionGroup:     regionGroup,
			PriceAmount:     decimal.RequireFromString("0.0000100"),
			Unit:            "s",
			PriceCurrency:   "USD",
			ServerlessRateAttributes: domain.ServerlessRateAttributes{
				Architecture:  domain.ArchitectureX86_64,
				Tier:          domain.ServerlessTierConsumption,
				ComponentType: domain.ComponentTypeDurationFeeCPU,
				Unit:          domain.UnitPerGHzSecond,
			},
			FetchedAt: fixedTime,
		},
		{
			Provider:        "gcp",
			ServiceCategory: "serverless",
			SkuID:           "SKU-GCP-MEM",
			Region:          "us-east4",
			RegionGroup:     regionGroup,
			PriceAmount:     decimal.RequireFromString("0.0000025"),
			Unit:            "GiBy.s",
			PriceCurrency:   "USD",
			ServerlessRateAttributes: domain.ServerlessRateAttributes{
				Architecture:  domain.ArchitectureX86_64,
				Tier:          domain.ServerlessTierConsumption,
				ComponentType: domain.ComponentTypeDurationFeeMemory,
				Unit:          domain.UnitPerGBSecond,
			},
			FetchedAt: fixedTime,
		},
	}
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "gcp", "serverless", "us-east4"), gcpObs, cache.DefaultTTL)

	pricingSvc := service.NewPricingService(nil, rdb)
	handler := rest.NewServerlessHandler(pricingSvc)

	// 1. x86_64 comparison request
	req := httptest.NewRequest(http.MethodGet, "/api/v1/prices/serverless?architecture=x86_64&requests_per_month=5000000&memory_mb=512&execution_duration_ms=200&region=us-east", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 OK: %s", rec.Code, rec.Body.String())
	}

	var resp rest.ServerlessComparisonResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(resp.Results))
	}

	// 2. arm64 comparison request -> AWS included, Azure & GCP excluded with warning
	reqARM := httptest.NewRequest(http.MethodGet, "/api/v1/prices/serverless?architecture=arm64&requests_per_month=5000000&memory_mb=512&execution_duration_ms=200&region=us-east", nil)
	recARM := httptest.NewRecorder()

	handler.ServeHTTP(recARM, reqARM)

	if recARM.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 OK: %s", recARM.Code, recARM.Body.String())
	}

	var respARM rest.ServerlessComparisonResponse
	if err := json.NewDecoder(recARM.Body).Decode(&respARM); err != nil {
		t.Fatalf("failed to decode arm64 response: %v", err)
	}

	if len(respARM.Results) != 1 {
		t.Fatalf("expected 1 result for arm64, got %d", len(respARM.Results))
	}
	if respARM.Results[0].Provider != "aws" {
		t.Errorf("expected AWS result for arm64, got %s", respARM.Results[0].Provider)
	}

	var azureExcluded, gcpExcluded bool
	for _, w := range respARM.Warnings {
		if w.Code == "architecture_unsupported_excluded" {
			if w.Provider == "azure" {
				azureExcluded = true
			}
			if w.Provider == "gcp" {
				gcpExcluded = true
			}
		}
	}
	if !azureExcluded || !gcpExcluded {
		t.Errorf("expected architecture_unsupported_excluded warnings for Azure and GCP, got %v", respARM.Warnings)
	}
}

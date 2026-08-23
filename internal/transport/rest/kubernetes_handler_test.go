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

func TestKubernetesHandler_InvalidParameters_ReturnsRFC7807(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	pricingSvc := service.NewPricingService(nil, rdb)
	handler := rest.NewKubernetesHandler(pricingSvc)

	tests := []struct {
		name       string
		query      string
		wantDetail string
	}{
		{
			name:       "invalid tier",
			query:      "tier=super_enterprise_tier",
			wantDetail: "tier must be a supported Kubernetes tier",
		},
		{
			name:       "invalid cluster_topology",
			query:      "cluster_topology=multi_cloud",
			wantDetail: "cluster_topology must be one of: zonal, regional, autopilot",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/prices/kubernetes?"+tt.query, nil)
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

func TestKubernetesHandler_MethodNotAllowed(t *testing.T) {
	pricingSvc := service.NewPricingService(nil, nil)
	handler := rest.NewKubernetesHandler(pricingSvc)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/prices/kubernetes", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405 Method Not Allowed", rec.Code)
	}
}

func TestKubernetesHandler_Success_Comparison(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	ctx := context.Background()
	fixedTime := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)

	// Warm cache for AWS, Azure, GCP
	awsK8s := []domain.PriceObservation{
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
			FetchedAt: fixedTime,
		},
	}
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "aws", "kubernetes", "us-east-1"), awsK8s, cache.DefaultTTL)

	azureK8s := []domain.PriceObservation{
		{
			Provider:        "azure",
			ServiceCategory: "kubernetes",
			SkuID:           "SKU-AZURE-AKS-STD",
			Region:          "eastus",
			RegionGroup:     "us-east",
			PriceAmount:     decimal.RequireFromString("0.10"),
			PriceCurrency:   "USD",
			Unit:            "Hrs",
			KubernetesAttributes: domain.KubernetesAttributes{
				Tier: "standard",
			},
			FetchedAt: fixedTime,
		},
	}
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "azure", "kubernetes", "eastus"), azureK8s, cache.DefaultTTL)

	gcpK8s := []domain.PriceObservation{
		{
			Provider:        "gcp",
			ServiceCategory: "kubernetes",
			SkuID:           "SKU-GCP-GKE-STD",
			Region:          "us-east4",
			RegionGroup:     "us-east",
			PriceAmount:     decimal.RequireFromString("0.10"),
			PriceCurrency:   "USD",
			Unit:            "hour",
			KubernetesAttributes: domain.KubernetesAttributes{
				Tier: "standard",
			},
			FetchedAt: fixedTime,
		},
	}
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "gcp", "kubernetes", "us-east4"), gcpK8s, cache.DefaultTTL)

	pricingSvc := service.NewPricingService(nil, rdb)
	handler := rest.NewKubernetesHandler(pricingSvc)

	// 1. Zonal topology query -> GKE credit applied
	req := httptest.NewRequest(http.MethodGet, "/api/v1/prices/kubernetes?tier=standard&cluster_topology=zonal&region=us-east", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 OK", rec.Code)
	}

	var resp rest.KubernetesComparisonResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(resp.Results))
	}

	resultsByProvider := make(map[string]rest.KubernetesResultEntry)
	for _, r := range resp.Results {
		resultsByProvider[r.Provider] = r
		if r.MatchQuality != "exact" {
			t.Errorf("provider %s match_quality = %s, want exact", r.Provider, r.MatchQuality)
		}
	}

	// AWS: $0.10/hr
	awsRes := resultsByProvider["aws"]
	if !awsRes.NormalizedHourlyUSD.Equal(decimal.RequireFromString("0.10")) {
		t.Errorf("AWS hourly cost = %s, want 0.10", awsRes.NormalizedHourlyUSD)
	}

	// Azure: $0.10/hr
	azureRes := resultsByProvider["azure"]
	if !azureRes.NormalizedHourlyUSD.Equal(decimal.RequireFromString("0.10")) {
		t.Errorf("Azure hourly cost = %s, want 0.10", azureRes.NormalizedHourlyUSD)
	}

	// GCP (Zonal): credit applied -> $0.00/hr
	gcpRes := resultsByProvider["gcp"]
	if !gcpRes.NormalizedHourlyUSD.IsZero() {
		t.Errorf("GCP zonal hourly cost = %s, want 0.00 (credit applied)", gcpRes.NormalizedHourlyUSD)
	}

	// 2. Regional topology query -> GKE credit NOT applied
	reqReg := httptest.NewRequest(http.MethodGet, "/api/v1/prices/kubernetes?tier=standard&cluster_topology=regional&region=us-east", nil)
	recReg := httptest.NewRecorder()

	handler.ServeHTTP(recReg, reqReg)
	if recReg.Code != http.StatusOK {
		t.Fatalf("regional status = %d, want 200 OK", recReg.Code)
	}

	var respReg rest.KubernetesComparisonResponse
	if err := json.NewDecoder(recReg.Body).Decode(&respReg); err != nil {
		t.Fatalf("failed to decode regional response: %v", err)
	}

	for _, r := range respReg.Results {
		if r.Provider == "gcp" {
			if !r.NormalizedHourlyUSD.Equal(decimal.RequireFromString("0.10")) {
				t.Errorf("GCP regional hourly cost = %s, want 0.10", r.NormalizedHourlyUSD)
			}
		}
	}
}

func TestKubernetesHandler_NonUSDCurrencyWarning(t *testing.T) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	ctx := context.Background()
	awsK8s := []domain.PriceObservation{
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
	_ = cache.Warm(ctx, rdb, cache.BuildKey(cache.SchemaVersion, "aws", "kubernetes", "us-east-1"), awsK8s, time.Hour)

	pricingSvc := service.NewPricingService(nil, rdb)
	handler := rest.NewKubernetesHandler(pricingSvc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/prices/kubernetes?currency=EUR", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 OK: %s", rec.Code, rec.Body.String())
	}

	var resp rest.KubernetesComparisonResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	var foundCurrencyWarning bool
	for _, w := range resp.Warnings {
		if w.Code == "non_usd_currency_unsupported" {
			foundCurrencyWarning = true
			break
		}
	}
	if !foundCurrencyWarning {
		t.Errorf("expected non_usd_currency_unsupported warning")
	}
}

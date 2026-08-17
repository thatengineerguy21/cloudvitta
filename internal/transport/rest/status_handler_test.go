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
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
	"github.com/thatengineerguy21/CloudVitta/internal/cache"
	"github.com/thatengineerguy21/CloudVitta/internal/dlq"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/service"
	"github.com/thatengineerguy21/CloudVitta/internal/store"
	"github.com/thatengineerguy21/CloudVitta/internal/transport/rest"
	"github.com/thatengineerguy21/CloudVitta/internal/transport/rest/middleware"
)

type mockStatusDLQ struct {
	getFunc func(ctx context.Context, provider, category string) (dlq.Entry, error)
}

func (m *mockStatusDLQ) Get(ctx context.Context, provider, category string) (dlq.Entry, error) {
	if m.getFunc != nil {
		return m.getFunc(ctx, provider, category)
	}
	return dlq.Entry{}, dlq.ErrEntryNotFound
}

func TestStatusHandler_ServeHTTP_HealthyProvider(t *testing.T) {
	fixedNow := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	fetchTime := fixedNow.Add(-1 * time.Hour)

	mockQ := &mockQuerier{
		getProviderCategoryStatusFunc: func(ctx context.Context, provider string) ([]store.GetProviderCategoryStatusRow, error) {
			if provider != "aws" {
				return nil, nil
			}
			return []store.GetProviderCategoryStatusRow{
				{
					ServiceCategory:  "compute",
					LastFetchedAt:    pgtype.Timestamptz{Time: fetchTime, Valid: true},
					LastSeenAt:       pgtype.Timestamptz{Time: fetchTime, Valid: true},
					ObservationCount: 450,
				},
				{
					ServiceCategory:  "storage",
					LastFetchedAt:    pgtype.Timestamptz{Time: fetchTime, Valid: true},
					LastSeenAt:       pgtype.Timestamptz{Time: fetchTime, Valid: true},
					ObservationCount: 120,
				},
				{
					ServiceCategory:  "network",
					LastFetchedAt:    pgtype.Timestamptz{Time: fetchTime, Valid: true},
					LastSeenAt:       pgtype.Timestamptz{Time: fetchTime, Valid: true},
					ObservationCount: 35,
				},
			}, nil
		},
	}

	freshnessSvc := service.NewFreshnessService(
		mockQ,
		nil,
		service.WithNowFunc(func() time.Time { return fixedNow }),
	)

	handler := rest.NewStatusHandler(freshnessSvc)

	mux := http.NewServeMux()
	mux.Handle("GET /api/v1/providers/{provider}/status", handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/providers/aws/status", nil)
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var status domain.ProviderStatus
	if err := json.NewDecoder(rr.Body).Decode(&status); err != nil {
		t.Fatalf("failed to decode JSON body: %v", err)
	}

	if status.Provider != "aws" {
		t.Errorf("expected provider 'aws', got %s", status.Provider)
	}
	if status.Status != "healthy" {
		t.Errorf("expected status 'healthy', got %s", status.Status)
	}
	if status.Stale {
		t.Errorf("expected Stale=false, got true")
	}
	if status.LastSuccessfulFetch == nil || !status.LastSuccessfulFetch.Equal(fetchTime) {
		t.Errorf("expected LastSuccessfulFetch %v, got %v", fetchTime, status.LastSuccessfulFetch)
	}
	if len(status.Categories) != 3 {
		t.Errorf("expected 3 categories, got %d", len(status.Categories))
	}
}

func TestStatusHandler_ServeHTTP_DegradedProviderWithDLQ(t *testing.T) {
	fixedNow := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	fetchTime := fixedNow.Add(-1 * time.Hour)

	mockQ := &mockQuerier{
		getProviderCategoryStatusFunc: func(ctx context.Context, provider string) ([]store.GetProviderCategoryStatusRow, error) {
			return []store.GetProviderCategoryStatusRow{
				{
					ServiceCategory:  "compute",
					LastFetchedAt:    pgtype.Timestamptz{Time: fetchTime, Valid: true},
					LastSeenAt:       pgtype.Timestamptz{Time: fetchTime, Valid: true},
					ObservationCount: 300,
				},
				{
					ServiceCategory:  "storage",
					LastFetchedAt:    pgtype.Timestamptz{Time: fixedNow.Add(-200 * time.Hour), Valid: true},
					LastSeenAt:       pgtype.Timestamptz{Time: fixedNow.Add(-200 * time.Hour), Valid: true},
					ObservationCount: 80,
				},
			}, nil
		},
	}

	mockDLQ := &mockStatusDLQ{
		getFunc: func(ctx context.Context, provider, category string) (dlq.Entry, error) {
			if provider == "azure" && category == "storage" {
				return dlq.Entry{
					Provider:            "azure",
					Category:            "storage",
					Timestamp:           fixedNow.Add(-5 * time.Minute),
					LastError:           "azure retail prices API returned 503 Service Unavailable",
					ConsecutiveFailures: 3,
					Status:              "failed",
				}, nil
			}
			return dlq.Entry{}, dlq.ErrEntryNotFound
		},
	}

	freshnessSvc := service.NewFreshnessService(
		mockQ,
		mockDLQ,
		service.WithNowFunc(func() time.Time { return fixedNow }),
	)

	handler := rest.NewStatusHandler(freshnessSvc)

	mux := http.NewServeMux()
	mux.Handle("GET /api/v1/providers/{provider}/status", handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/providers/azure/status", nil)
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var status domain.ProviderStatus
	if err := json.NewDecoder(rr.Body).Decode(&status); err != nil {
		t.Fatalf("failed to decode JSON body: %v", err)
	}

	if status.Status != "degraded" {
		t.Errorf("expected status 'degraded', got %s", status.Status)
	}
	if !status.Stale {
		t.Errorf("expected Stale=true")
	}

	storageCat := status.Categories["storage"]
	if !storageCat.Stale {
		t.Errorf("expected storage Stale=true")
	}
	if storageCat.DLQ == nil {
		t.Fatalf("expected DLQ entry for storage")
	}
	if storageCat.DLQ.Status != "failed" || storageCat.DLQ.ConsecutiveFailures != 3 {
		t.Errorf("unexpected DLQ payload: %+v", storageCat.DLQ)
	}
}

func TestStatusHandler_ServeHTTP_Stage3Provider(t *testing.T) {
	freshnessSvc := service.NewFreshnessService(nil, nil)
	handler := rest.NewStatusHandler(freshnessSvc)

	mux := http.NewServeMux()
	mux.Handle("GET /api/v1/providers/{provider}/status", handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/providers/oracle/status", nil)
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}

	var status domain.ProviderStatus
	if err := json.NewDecoder(rr.Body).Decode(&status); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}

	if status.Status != "not_yet_ingested" {
		t.Errorf("expected status 'not_yet_ingested', got %s", status.Status)
	}
	if !status.Stale {
		t.Errorf("expected Stale=true for stage 3 provider")
	}
	if len(status.Warnings) != 1 || status.Warnings[0].Code != "not_yet_ingested" {
		t.Errorf("expected not_yet_ingested warning, got %+v", status.Warnings)
	}
}

func TestStatusHandler_ServeHTTP_UnknownProvider_RFC7807(t *testing.T) {
	freshnessSvc := service.NewFreshnessService(nil, nil)
	handler := rest.NewStatusHandler(freshnessSvc)

	mux := http.NewServeMux()
	mux.Handle("GET /api/v1/providers/{provider}/status", handler)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/providers/unknown-cloud/status", nil)
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rr.Code)
	}

	var errResp middleware.RFC7807Error
	if err := json.NewDecoder(rr.Body).Decode(&errResp); err != nil {
		t.Fatalf("failed to decode RFC7807 error: %v", err)
	}

	if errResp.Type != "https://cloudvitta.dev/errors/provider-not-found" {
		t.Errorf("expected RFC7807 type 'https://cloudvitta.dev/errors/provider-not-found', got %s", errResp.Type)
	}
	if errResp.Status != 404 {
		t.Errorf("expected RFC7807 status 404, got %d", errResp.Status)
	}
	if !strings.Contains(errResp.Detail, "unknown-cloud") {
		t.Errorf("expected detail mentioning unknown-cloud, got %s", errResp.Detail)
	}
}

func TestCrossEndpoint_StalenessAgreement(t *testing.T) {
	fixedNow := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	staleFetchTime := fixedNow.Add(-200 * time.Hour) // > 168h default

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis.Run() failed: %v", err)
	}
	defer mr.Close()

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer func() { _ = rdb.Close() }()

	ctx := context.Background()

	// Seed Stale AWS Compute observation in cache
	staleObs := []domain.PriceObservation{
		{
			Provider:          "aws",
			ServiceCategory:   "compute",
			SkuID:             "t3.medium",
			DisplayName:       "t3.medium",
			Region:            "us-east-1",
			RegionGroup:       "us-east",
			Unit:              "hour",
			PriceAmount:       decimal.RequireFromString("0.0416"),
			PriceCurrency:     "USD",
			PricingModel:      "on-demand",
			Attributes:        domain.ComputeAttributes{VCPU: 2, RAMGB: 4, Family: "t3"},
			StorageAttributes: domain.StorageAttributes{},
			NetworkAttributes: domain.NetworkAttributes{},
			FetchedAt:         staleFetchTime,
		},
	}

	staleKey := cache.BuildKey(cache.SchemaVersion, "aws", "compute", "us-east-1")
	if err := cache.Warm(ctx, rdb, staleKey, staleObs, cache.DefaultTTL); err != nil {
		t.Fatalf("cache.Warm failed: %v", err)
	}

	mockQ := &mockQuerier{
		getProviderCategoryStatusFunc: func(ctx context.Context, provider string) ([]store.GetProviderCategoryStatusRow, error) {
			if provider == "aws" {
				return []store.GetProviderCategoryStatusRow{
					{
						ServiceCategory:  "compute",
						LastFetchedAt:    pgtype.Timestamptz{Time: staleFetchTime, Valid: true},
						LastSeenAt:       pgtype.Timestamptz{Time: staleFetchTime, Valid: true},
						ObservationCount: 1,
					},
					{
						ServiceCategory:  "storage",
						LastFetchedAt:    pgtype.Timestamptz{Time: staleFetchTime, Valid: true},
						LastSeenAt:       pgtype.Timestamptz{Time: staleFetchTime, Valid: true},
						ObservationCount: 1,
					},
					{
						ServiceCategory:  "network",
						LastFetchedAt:    pgtype.Timestamptz{Time: staleFetchTime, Valid: true},
						LastSeenAt:       pgtype.Timestamptz{Time: staleFetchTime, Valid: true},
						ObservationCount: 1,
					},
				}, nil
			}
			return nil, nil
		},
	}

	freshnessSvc := service.NewFreshnessService(
		mockQ,
		nil,
		service.WithNowFunc(func() time.Time { return fixedNow }),
	)

	pricingSvc := service.NewPricingService(
		nil,
		rdb,
		service.WithFreshnessService(freshnessSvc),
	)

	computeHandler := rest.NewComputeHandler(pricingSvc)
	calcHandler := rest.NewCalculateHandler(pricingSvc)
	statusHandler := rest.NewStatusHandler(freshnessSvc)

	mux := http.NewServeMux()
	mux.Handle("GET /api/v1/prices/compute", computeHandler)
	mux.Handle("POST /api/v1/calculate", calcHandler)
	mux.Handle("GET /api/v1/providers/{provider}/status", statusHandler)

	// 1. Verify GET /api/v1/prices/compute reports stale: true
	{
		req := httptest.NewRequest(http.MethodGet, "/api/v1/prices/compute?vcpu=2&ram_gb=4&region=us-east", nil)
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("compute endpoint status %d: %s", rr.Code, rr.Body.String())
		}

		var compResp struct {
			Results []struct {
				Provider string `json:"provider"`
				Stale    bool   `json:"stale"`
			} `json:"results"`
		}
		if err := json.NewDecoder(rr.Body).Decode(&compResp); err != nil {
			t.Fatalf("decode compute JSON failed: %v", err)
		}
		if len(compResp.Results) == 0 {
			t.Fatalf("expected compute results, got 0")
		}
		if !compResp.Results[0].Stale {
			t.Errorf("expected compute result stale == true for 200h old data, got false")
		}
	}

	// 2. Verify POST /api/v1/calculate reports compute category stale: true
	{
		calcBody := `{"region":"us-east","compute":{"vcpu":2,"ram_gb":4}}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/calculate", strings.NewReader(calcBody))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("calculate endpoint status %d: %s", rr.Code, rr.Body.String())
		}

		var calcResp struct {
			Results []struct {
				Provider   string `json:"provider"`
				Categories map[string]struct {
					Stale bool `json:"stale"`
				} `json:"categories"`
			} `json:"results"`
		}
		if err := json.NewDecoder(rr.Body).Decode(&calcResp); err != nil {
			t.Fatalf("decode calculate JSON failed: %v", err)
		}
		if len(calcResp.Results) == 0 {
			t.Fatalf("expected calculate results, got 0")
		}
		awsRes := calcResp.Results[0]
		if !awsRes.Categories["compute"].Stale {
			t.Errorf("expected calculate result compute category stale == true, got false")
		}
	}

	// 3. Verify GET /api/v1/providers/aws/status reports compute category and overall stale: true
	{
		req := httptest.NewRequest(http.MethodGet, "/api/v1/providers/aws/status", nil)
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("status endpoint status %d: %s", rr.Code, rr.Body.String())
		}

		var statusResp domain.ProviderStatus
		if err := json.NewDecoder(rr.Body).Decode(&statusResp); err != nil {
			t.Fatalf("decode status JSON failed: %v", err)
		}
		if !statusResp.Stale {
			t.Errorf("expected provider status stale == true, got false")
		}
		if !statusResp.Categories["compute"].Stale {
			t.Errorf("expected provider status compute category stale == true, got false")
		}
	}
}

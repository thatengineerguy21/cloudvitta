package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/thatengineerguy21/CloudVitta/internal/dlq"
	"github.com/thatengineerguy21/CloudVitta/internal/service"
	"github.com/thatengineerguy21/CloudVitta/internal/store"
)

type mockDLQReader struct {
	getFunc func(ctx context.Context, provider, category string) (dlq.Entry, error)
}

func (m *mockDLQReader) Get(ctx context.Context, provider, category string) (dlq.Entry, error) {
	if m.getFunc != nil {
		return m.getFunc(ctx, provider, category)
	}
	return dlq.Entry{}, dlq.ErrEntryNotFound
}

type mockStatusQuerier struct {
	getProviderCategoryStatusFunc func(ctx context.Context, provider string) ([]store.GetProviderCategoryStatusRow, error)
}

func (m *mockStatusQuerier) CreateUser(ctx context.Context, arg store.CreateUserParams) (store.User, error) {
	return store.User{}, errors.New("not implemented")
}

func (m *mockStatusQuerier) GetUserByEmail(ctx context.Context, email string) (store.User, error) {
	return store.User{}, errors.New("not implemented")
}

func (m *mockStatusQuerier) GetUserByID(ctx context.Context, id pgtype.UUID) (store.User, error) {
	return store.User{}, errors.New("not implemented")
}

func (m *mockStatusQuerier) InsertRefreshToken(ctx context.Context, arg store.InsertRefreshTokenParams) (store.RefreshToken, error) {
	return store.RefreshToken{}, errors.New("not implemented")
}

func (m *mockStatusQuerier) GetRefreshTokenByHashForUpdate(ctx context.Context, tokenHash string) (store.RefreshToken, error) {
	return store.RefreshToken{}, errors.New("not implemented")
}

func (m *mockStatusQuerier) GetRefreshTokenByID(ctx context.Context, id pgtype.UUID) (store.RefreshToken, error) {
	return store.RefreshToken{}, errors.New("not implemented")
}

func (m *mockStatusQuerier) RevokeRefreshTokenWithReplacement(ctx context.Context, arg store.RevokeRefreshTokenWithReplacementParams) error {
	return nil
}

func (m *mockStatusQuerier) RevokeRefreshTokenByHash(ctx context.Context, arg store.RevokeRefreshTokenByHashParams) error {
	return nil
}

func (m *mockStatusQuerier) RevokeRefreshTokenFamily(ctx context.Context, arg store.RevokeRefreshTokenFamilyParams) error {
	return nil
}

func (m *mockStatusQuerier) ListRefreshTokensByFamilyID(ctx context.Context, familyID pgtype.UUID) ([]store.RefreshToken, error) {
	return nil, nil
}

func (m *mockStatusQuerier) GetPriceObservations(ctx context.Context, arg store.GetPriceObservationsParams) ([]store.PriceObservation, error) {
	return nil, nil
}

func (m *mockStatusQuerier) GetLatestPriceForSKU(ctx context.Context, arg store.GetLatestPriceForSKUParams) (store.PriceObservation, error) {
	return store.PriceObservation{}, nil
}

func (m *mockStatusQuerier) GetLatestPriceForSKUAndCategory(ctx context.Context, arg store.GetLatestPriceForSKUAndCategoryParams) (store.PriceObservation, error) {
	return store.PriceObservation{}, nil
}

func (m *mockStatusQuerier) UpdatePriceObservationLastSeenAt(ctx context.Context, arg store.UpdatePriceObservationLastSeenAtParams) error {
	return nil
}

func (m *mockStatusQuerier) InsertPriceObservation(ctx context.Context, arg store.InsertPriceObservationParams) (int64, error) {
	return 0, nil
}

func (m *mockStatusQuerier) GetProviderCategoryStatus(ctx context.Context, provider string) ([]store.GetProviderCategoryStatusRow, error) {
	if m.getProviderCategoryStatusFunc != nil {
		return m.getProviderCategoryStatusFunc(ctx, provider)
	}
	return nil, nil
}

func TestFreshnessService_IsStale_Boundaries(t *testing.T) {
	fixedNow := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	svc := service.NewFreshnessService(
		nil,
		nil,
		service.WithNowFunc(func() time.Time { return fixedNow }),
		service.WithDefaultThreshold(168*time.Hour),
		service.WithCustomThreshold("aws:compute", 24*time.Hour),
	)

	tests := []struct {
		name      string
		provider  string
		category  string
		fetchedAt time.Time
		wantStale bool
	}{
		{
			name:      "zero time is always stale",
			provider:  "aws",
			category:  "storage",
			fetchedAt: time.Time{},
			wantStale: true,
		},
		{
			name:      "fresh default threshold (168h - 1s)",
			provider:  "aws",
			category:  "storage",
			fetchedAt: fixedNow.Add(-168*time.Hour + time.Second),
			wantStale: false,
		},
		{
			name:      "exact boundary default threshold (168h) is not stale",
			provider:  "aws",
			category:  "storage",
			fetchedAt: fixedNow.Add(-168 * time.Hour),
			wantStale: false,
		},
		{
			name:      "stale default threshold (168h + 1s)",
			provider:  "aws",
			category:  "storage",
			fetchedAt: fixedNow.Add(-168*time.Hour - time.Second),
			wantStale: true,
		},
		{
			name:      "custom threshold fresh (24h - 1s)",
			provider:  "aws",
			category:  "compute",
			fetchedAt: fixedNow.Add(-24*time.Hour + time.Second),
			wantStale: false,
		},
		{
			name:      "custom threshold stale (24h + 1s)",
			provider:  "aws",
			category:  "compute",
			fetchedAt: fixedNow.Add(-24*time.Hour - time.Second),
			wantStale: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := svc.IsStale(tt.provider, tt.category, tt.fetchedAt)
			if got != tt.wantStale {
				t.Errorf("IsStale(%s, %s, %v) = %v; want %v", tt.provider, tt.category, tt.fetchedAt, got, tt.wantStale)
			}
		})
	}
}

func TestFreshnessService_GetProviderStatus_Healthy(t *testing.T) {
	fixedNow := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	fetchTime := fixedNow.Add(-2 * time.Hour)

	mockQ := &mockStatusQuerier{
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

	svc := service.NewFreshnessService(
		mockQ,
		nil,
		service.WithNowFunc(func() time.Time { return fixedNow }),
	)

	status, err := svc.GetProviderStatus(context.Background(), "aws")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if status.Provider != "aws" {
		t.Errorf("expected provider 'aws', got %s", status.Provider)
	}
	if status.Status != "healthy" {
		t.Errorf("expected status 'healthy', got %s", status.Status)
	}
	if status.Stale {
		t.Errorf("expected Stale=false for healthy provider, got true")
	}
	if status.LastSuccessfulFetch == nil || !status.LastSuccessfulFetch.Equal(fetchTime) {
		t.Errorf("expected LastSuccessfulFetch %v, got %v", fetchTime, status.LastSuccessfulFetch)
	}
	if len(status.Categories) != 3 {
		t.Fatalf("expected 3 categories, got %d", len(status.Categories))
	}
	computeCat := status.Categories["compute"]
	if computeCat.ObservationCount != 450 || computeCat.Stale || computeCat.DLQ != nil {
		t.Errorf("unexpected compute category status: %+v", computeCat)
	}
}

func TestFreshnessService_GetProviderStatus_DegradedDLQ(t *testing.T) {
	fixedNow := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	fetchTime := fixedNow.Add(-2 * time.Hour)

	mockQ := &mockStatusQuerier{
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

	mockDLQ := &mockDLQReader{
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

	svc := service.NewFreshnessService(
		mockQ,
		mockDLQ,
		service.WithNowFunc(func() time.Time { return fixedNow }),
	)

	status, err := svc.GetProviderStatus(context.Background(), "azure")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if status.Status != "degraded" {
		t.Errorf("expected status 'degraded', got %s", status.Status)
	}
	if !status.Stale {
		t.Errorf("expected Stale=true when a category is stale/errored")
	}

	storageCat := status.Categories["storage"]
	if !storageCat.Stale {
		t.Errorf("expected storage category Stale=true")
	}
	if storageCat.DLQ == nil {
		t.Fatalf("expected storage DLQ entry to be populated")
	}
	if storageCat.DLQ.Status != "failed" || storageCat.DLQ.ConsecutiveFailures != 3 {
		t.Errorf("unexpected storage DLQ: %+v", storageCat.DLQ)
	}
}

func TestFreshnessService_GetProviderStatus_Blocked(t *testing.T) {
	fixedNow := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	mockQ := &mockStatusQuerier{}
	mockDLQ := &mockDLQReader{
		getFunc: func(ctx context.Context, provider, category string) (dlq.Entry, error) {
			if category == "compute" {
				return dlq.Entry{
					Provider:            provider,
					Category:            "compute",
					Timestamp:           fixedNow,
					LastError:           "invalid credentials 401 Unauthorized",
					ConsecutiveFailures: 5,
					Status:              "blocked",
				}, nil
			}
			return dlq.Entry{}, dlq.ErrEntryNotFound
		},
	}

	svc := service.NewFreshnessService(
		mockQ,
		mockDLQ,
		service.WithNowFunc(func() time.Time { return fixedNow }),
	)

	status, err := svc.GetProviderStatus(context.Background(), "gcp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if status.Status != "blocked" {
		t.Errorf("expected status 'blocked', got %s", status.Status)
	}
	if !status.Stale {
		t.Errorf("expected Stale=true for blocked provider")
	}
}

func TestFreshnessService_GetProviderStatus_StaleAll(t *testing.T) {
	fixedNow := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	staleTime := fixedNow.Add(-300 * time.Hour) // > 168h default

	mockQ := &mockStatusQuerier{
		getProviderCategoryStatusFunc: func(ctx context.Context, provider string) ([]store.GetProviderCategoryStatusRow, error) {
			return []store.GetProviderCategoryStatusRow{
				{
					ServiceCategory:  "compute",
					LastFetchedAt:    pgtype.Timestamptz{Time: staleTime, Valid: true},
					LastSeenAt:       pgtype.Timestamptz{Time: staleTime, Valid: true},
					ObservationCount: 100,
				},
				{
					ServiceCategory:  "storage",
					LastFetchedAt:    pgtype.Timestamptz{Time: staleTime, Valid: true},
					LastSeenAt:       pgtype.Timestamptz{Time: staleTime, Valid: true},
					ObservationCount: 50,
				},
				{
					ServiceCategory:  "network",
					LastFetchedAt:    pgtype.Timestamptz{Time: staleTime, Valid: true},
					LastSeenAt:       pgtype.Timestamptz{Time: staleTime, Valid: true},
					ObservationCount: 20,
				},
			}, nil
		},
	}

	svc := service.NewFreshnessService(
		mockQ,
		nil,
		service.WithNowFunc(func() time.Time { return fixedNow }),
	)

	status, err := svc.GetProviderStatus(context.Background(), "aws")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if status.Status != "stale" {
		t.Errorf("expected status 'stale', got %s", status.Status)
	}
	if !status.Stale {
		t.Errorf("expected Stale=true for stale provider")
	}
}

func TestFreshnessService_GetProviderStatus_Stage3Providers(t *testing.T) {
	svc := service.NewFreshnessService(nil, nil)

	stage3 := []struct {
		provider string
		msg      string
	}{
		{"oracle", "Oracle OCI ingestion lands in stage 3."},
		{"ibm", "IBM Cloud ingestion lands in stage 3."},
		{"alibaba", "Alibaba Cloud ingestion lands in stage 3."},
		{"digitalocean", "DigitalOcean ingestion lands in stage 3."},
	}

	for _, s3 := range stage3 {
		t.Run(s3.provider, func(t *testing.T) {
			status, err := svc.GetProviderStatus(context.Background(), s3.provider)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if status.Status != "not_yet_ingested" {
				t.Errorf("expected status 'not_yet_ingested', got %s", status.Status)
			}
			if !status.Stale {
				t.Errorf("expected Stale=true for stage 3 provider")
			}
			if len(status.Warnings) != 1 {
				t.Fatalf("expected 1 warning, got %d", len(status.Warnings))
			}
			if status.Warnings[0].Code != "not_yet_ingested" || status.Warnings[0].Message != s3.msg {
				t.Errorf("unexpected warning: %+v", status.Warnings[0])
			}
		})
	}
}

func TestFreshnessService_GetProviderStatus_UnknownProvider(t *testing.T) {
	svc := service.NewFreshnessService(nil, nil)

	_, err := svc.GetProviderStatus(context.Background(), "unknown-cloud")
	if !errors.Is(err, service.ErrProviderNotFound) {
		t.Fatalf("expected ErrProviderNotFound, got %v", err)
	}
}

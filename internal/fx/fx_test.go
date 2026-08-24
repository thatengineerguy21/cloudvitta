package fx_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/thatengineerguy21/CloudVitta/internal/fx"
	"github.com/thatengineerguy21/CloudVitta/internal/fx/frankfurter"
	"github.com/thatengineerguy21/CloudVitta/internal/store"
)

// mockQuerier implements store.Querier for testing DB interactions.
type mockQuerier struct {
	store.Querier
	mu          sync.Mutex
	upsertCalls []store.UpsertFXRateParams
	listRates   []store.FxRate
	listErr     error
	getRateFunc func(ctx context.Context, arg store.GetLatestFXRateParams) (store.FxRate, error)
}

func (m *mockQuerier) UpsertFXRate(ctx context.Context, arg store.UpsertFXRateParams) (store.FxRate, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.upsertCalls = append(m.upsertCalls, arg)
	return store.FxRate{}, nil
}

func (m *mockQuerier) ListLatestFXRates(ctx context.Context, baseCurrency string) ([]store.FxRate, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.listRates, nil
}

func (m *mockQuerier) GetLatestFXRate(ctx context.Context, arg store.GetLatestFXRateParams) (store.FxRate, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.getRateFunc != nil {
		return m.getRateFunc(ctx, arg)
	}
	return store.FxRate{}, errors.New("GetLatestFXRate not found")
}

// mockClient implements RateFetcher for testing.
type mockClient struct {
	resp *frankfurter.RatesResponse
	err  error
}

func (m *mockClient) FetchLatestRates(ctx context.Context, baseCurrency string) (*frankfurter.RatesResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.resp, nil
}

func TestNoOpFXService(t *testing.T) {
	svc := fx.NewNoOpFXService()
	ctx := context.Background()

	// Identity conversion
	amt := decimal.NewFromFloat(100.50)
	converted, meta, err := svc.Convert(ctx, amt, "USD", "USD")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !converted.Equal(amt) {
		t.Errorf("expected %s, got %s", amt, converted)
	}
	if !meta.Rate.Equal(decimal.NewFromInt(1)) {
		t.Errorf("expected rate 1, got %s", meta.Rate)
	}

	// Cross currency unsupported
	_, _, err = svc.Convert(ctx, amt, "USD", "EUR")
	if !errors.Is(err, fx.ErrCurrencyConversionUnsupported) {
		t.Errorf("expected ErrCurrencyConversionUnsupported, got %v", err)
	}

	// GetRate
	rate, _, err := svc.GetRate(ctx, "EUR", "EUR")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !rate.Equal(decimal.NewFromInt(1)) {
		t.Errorf("expected rate 1, got %s", rate)
	}

	_, _, err = svc.GetRate(ctx, "USD", "CNY")
	if !errors.Is(err, fx.ErrCurrencyConversionUnsupported) {
		t.Errorf("expected ErrCurrencyConversionUnsupported, got %v", err)
	}

	if err := svc.RefreshRates(ctx); err != nil {
		t.Errorf("unexpected error on RefreshRates: %v", err)
	}
}

func sampleRates() map[string]fx.CachedRate {
	fixedTime := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	return map[string]fx.CachedRate{
		"USD": {
			Rate:      decimal.NewFromInt(1),
			Source:    "base",
			RateDate:  "2026-08-21",
			FetchedAt: fixedTime,
		},
		"EUR": {
			Rate:      decimal.RequireFromString("0.9000"), // 1 USD = 0.90 EUR
			Source:    "frankfurter",
			RateDate:  "2026-08-21",
			FetchedAt: fixedTime,
		},
		"CNY": {
			Rate:      decimal.RequireFromString("7.2000"), // 1 USD = 7.20 CNY
			Source:    "frankfurter",
			RateDate:  "2026-08-21",
			FetchedAt: fixedTime,
		},
		"GBP": {
			Rate:      decimal.RequireFromString("0.8000"), // 1 USD = 0.80 GBP
			Source:    "frankfurter",
			RateDate:  "2026-08-21",
			FetchedAt: fixedTime,
		},
	}
}

func TestFXService_Conversions(t *testing.T) {
	ctx := context.Background()
	svc := fx.NewService(nil, nil, fx.WithInitialRates(sampleRates()))

	tests := []struct {
		name         string
		amount       decimal.Decimal
		from         string
		to           string
		expectedAmt  decimal.Decimal
		expectedRate decimal.Decimal
		expectedMeta string
		expectErr    error
	}{
		{
			name:         "Identity USD -> USD",
			amount:       decimal.NewFromInt(100),
			from:         "USD",
			to:           "USD",
			expectedAmt:  decimal.NewFromInt(100),
			expectedRate: decimal.NewFromInt(1),
		},
		{
			name:         "Direct USD -> EUR",
			amount:       decimal.NewFromInt(100),
			from:         "USD",
			to:           "EUR",
			expectedAmt:  decimal.NewFromFloat(90.00),
			expectedRate: decimal.RequireFromString("0.9000"),
		},
		{
			name:         "Direct USD -> CNY",
			amount:       decimal.NewFromInt(100),
			from:         "USD",
			to:           "CNY",
			expectedAmt:  decimal.NewFromInt(720),
			expectedRate: decimal.RequireFromString("7.2000"),
		},
		{
			name:         "Inverse CNY -> USD (720 CNY = 100 USD)",
			amount:       decimal.NewFromInt(720),
			from:         "CNY",
			to:           "USD",
			expectedAmt:  decimal.NewFromInt(100),
			expectedRate: decimal.NewFromInt(1).Div(decimal.RequireFromString("7.2000")),
		},
		{
			name:         "Cross CNY -> EUR (720 CNY -> 100 USD -> 90 EUR)",
			amount:       decimal.NewFromInt(720),
			from:         "CNY",
			to:           "EUR",
			expectedAmt:  decimal.NewFromFloat(90.00),
			expectedRate: decimal.RequireFromString("0.9000").Div(decimal.RequireFromString("7.2000")),
		},
		{
			name:         "Cross EUR -> GBP (90 EUR -> 100 USD -> 80 GBP)",
			amount:       decimal.NewFromFloat(90),
			from:         "EUR",
			to:           "GBP",
			expectedAmt:  decimal.NewFromInt(80),
			expectedRate: decimal.RequireFromString("0.8000").Div(decimal.RequireFromString("0.9000")),
		},
		{
			name:      "Negative amount error",
			amount:    decimal.NewFromInt(-50),
			from:      "USD",
			to:        "EUR",
			expectErr: fx.ErrNegativeAmount,
		},
		{
			name:      "Empty currency error",
			amount:    decimal.NewFromInt(100),
			from:      "",
			to:        "EUR",
			expectErr: fx.ErrInvalidCurrency,
		},
		{
			name:      "Unknown currency rate not found",
			amount:    decimal.NewFromInt(100),
			from:      "USD",
			to:        "XYZ",
			expectErr: fx.ErrRateNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			conv, meta, err := svc.Convert(ctx, tc.amount, tc.from, tc.to)
			if tc.expectErr != nil {
				if !errors.Is(err, tc.expectErr) {
					t.Fatalf("expected error %v, got %v", tc.expectErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !conv.Equal(tc.expectedAmt) {
				t.Errorf("expected converted amount %s, got %s", tc.expectedAmt, conv)
			}
			if !meta.Rate.Equal(tc.expectedRate) {
				t.Errorf("expected rate %s, got %s", tc.expectedRate, meta.Rate)
			}
		})
	}
}

func TestFXService_RefreshRates_Success(t *testing.T) {
	ctx := context.Background()

	mockCli := &mockClient{
		resp: &frankfurter.RatesResponse{
			Amount: decimal.NewFromInt(1),
			Base:   "USD",
			Date:   "2026-08-24",
			Rates: map[string]decimal.Decimal{
				"EUR": decimal.RequireFromString("0.9100"),
				"CNY": decimal.RequireFromString("7.1500"),
			},
		},
	}

	numEUR, _ := store.DecimalToNumeric(decimal.RequireFromString("0.9100"))
	mockQ := &mockQuerier{
		listRates: []store.FxRate{
			{
				BaseCurrency:   "USD",
				TargetCurrency: "EUR",
				Rate:           numEUR,
				Source:         "frankfurter",
			},
		},
	}

	svc := fx.NewService(mockQ, mockCli)

	err := svc.RefreshRates(ctx)
	if err != nil {
		t.Fatalf("unexpected refresh error: %v", err)
	}

	// Verify rates are cached and live
	rate, meta, err := svc.GetRate(ctx, "USD", "EUR")
	if err != nil {
		t.Fatalf("unexpected get rate error: %v", err)
	}
	if !rate.Equal(decimal.RequireFromString("0.9100")) {
		t.Errorf("expected EUR rate 0.9100, got %s", rate)
	}
	if meta.IsFallback {
		t.Errorf("expected IsFallback=false for live rate")
	}
	if meta.Source != "frankfurter" {
		t.Errorf("expected source frankfurter, got %s", meta.Source)
	}
}

func TestFXService_RefreshRates_UpstreamFailure_DBFallback(t *testing.T) {
	ctx := context.Background()

	mockCli := &mockClient{
		err: errors.New("connection refused"),
	}

	numCNY, _ := store.DecimalToNumeric(decimal.RequireFromString("7.2500"))
	d, _ := store.DateFromString("2026-08-20")
	now := time.Now().UTC()

	mockQ := &mockQuerier{
		listRates: []store.FxRate{
			{
				ID:             1,
				BaseCurrency:   "USD",
				TargetCurrency: "CNY",
				Rate:           numCNY,
				Source:         "frankfurter",
				RateDate:       d,
				FetchedAt:      store.TimestamptzFromTime(now),
			},
		},
	}

	svc := fx.NewService(mockQ, mockCli)

	err := svc.RefreshRates(ctx)
	if err != nil {
		t.Fatalf("expected fallback to succeed, got error: %v", err)
	}

	rate, meta, err := svc.GetRate(ctx, "USD", "CNY")
	if err != nil {
		t.Fatalf("unexpected get rate error: %v", err)
	}
	if !rate.Equal(decimal.RequireFromString("7.2500")) {
		t.Errorf("expected fallback rate 7.2500, got %s", rate)
	}
	if !meta.IsFallback {
		t.Errorf("expected IsFallback=true on database fallback")
	}
}

func TestFXService_RefreshRates_AllFailures(t *testing.T) {
	ctx := context.Background()

	mockCli := &mockClient{
		err: errors.New("upstream 503"),
	}
	mockQ := &mockQuerier{
		listErr: errors.New("db connection down"),
	}

	svc := fx.NewService(mockQ, mockCli)

	err := svc.RefreshRates(ctx)
	if !errors.Is(err, fx.ErrUpstreamUnavailable) {
		t.Fatalf("expected ErrUpstreamUnavailable, got %v", err)
	}
}

func TestFXService_LazyDBFallback_InverseAndCross(t *testing.T) {
	ctx := context.Background()

	numEUR, _ := store.DecimalToNumeric(decimal.RequireFromString("0.9000"))
	numCNY, _ := store.DecimalToNumeric(decimal.RequireFromString("7.2000"))
	rateDate, _ := store.DateFromString("2026-08-24")
	now := time.Now().UTC()

	mockQ := &mockQuerier{
		getRateFunc: func(ctx context.Context, arg store.GetLatestFXRateParams) (store.FxRate, error) {
			if arg.BaseCurrency == "USD" && arg.TargetCurrency == "EUR" {
				return store.FxRate{
					BaseCurrency:   "USD",
					TargetCurrency: "EUR",
					Rate:           numEUR,
					Source:         "frankfurter",
					RateDate:       rateDate,
					FetchedAt:      store.TimestamptzFromTime(now),
				}, nil
			}
			if arg.BaseCurrency == "USD" && arg.TargetCurrency == "CNY" {
				return store.FxRate{
					BaseCurrency:   "USD",
					TargetCurrency: "CNY",
					Rate:           numCNY,
					Source:         "frankfurter",
					RateDate:       rateDate,
					FetchedAt:      store.TimestamptzFromTime(now),
				}, nil
			}
			return store.FxRate{}, errors.New("rate not found")
		},
	}

	// Service without initial in-memory cache and without upstream client
	svc := fx.NewService(mockQ, nil)

	// 1. Direct USD -> EUR
	conv, meta, err := svc.Convert(ctx, decimal.NewFromInt(100), "USD", "EUR")
	if err != nil {
		t.Fatalf("unexpected direct conversion error: %v", err)
	}
	if !conv.Equal(decimal.NewFromFloat(90)) {
		t.Errorf("expected 90 EUR, got %s", conv)
	}
	if !meta.IsFallback {
		t.Errorf("expected IsFallback=true on DB fallback")
	}

	// 2. Inverse CNY -> USD (720 CNY -> 100 USD)
	conv, meta, err = svc.Convert(ctx, decimal.NewFromInt(720), "CNY", "USD")
	if err != nil {
		t.Fatalf("unexpected inverse conversion error: %v", err)
	}
	if !conv.Equal(decimal.NewFromInt(100)) {
		t.Errorf("expected 100 USD, got %s", conv)
	}
	if !meta.IsFallback {
		t.Errorf("expected IsFallback=true on DB fallback")
	}

	// 3. Cross CNY -> EUR (720 CNY -> 90 EUR)
	conv, meta, err = svc.Convert(ctx, decimal.NewFromInt(720), "CNY", "EUR")
	if err != nil {
		t.Fatalf("unexpected cross conversion error: %v", err)
	}
	if !conv.Equal(decimal.NewFromFloat(90)) {
		t.Errorf("expected 90 EUR, got %s", conv)
	}
	if !meta.IsFallback {
		t.Errorf("expected IsFallback=true on DB fallback")
	}
}

func TestFXService_Concurrency(t *testing.T) {
	ctx := context.Background()
	svc := fx.NewService(nil, nil, fx.WithInitialRates(sampleRates()))

	var wg sync.WaitGroup
	workers := 20
	iterations := 100

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				if j%10 == 0 {
					_ = svc.RefreshRates(ctx)
				}
				amt := decimal.NewFromInt(int64(j + 1))
				_, _, _ = svc.Convert(ctx, amt, "USD", "EUR")
				_, _, _ = svc.Convert(ctx, amt, "CNY", "USD")
				_, _, _ = svc.Convert(ctx, amt, "CNY", "EUR")
				_, _, _ = svc.GetRate(ctx, "USD", "GBP")
			}
		}(i)
	}

	wg.Wait()
}

package fx

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/shopspring/decimal"
	"github.com/thatengineerguy21/CloudVitta/internal/fx/frankfurter"
	"github.com/thatengineerguy21/CloudVitta/internal/store"
)

// RateFetcher queries upstream exchange rates.
type RateFetcher interface {
	FetchLatestRates(ctx context.Context, baseCurrency string) (*frankfurter.RatesResponse, error)
}

// CachedRate represents an exchange rate cached in memory.
type CachedRate struct {
	Rate       decimal.Decimal
	Source     string
	RateDate   string
	FetchedAt  time.Time
	IsFallback bool
}

// Service is the concrete implementation of FXService.
type Service struct {
	queries      store.Querier
	client       RateFetcher
	baseCurrency string
	clock        func() time.Time

	mu    sync.RWMutex
	rates map[string]CachedRate
}

// Option configures Service.
type Option func(*Service)

// WithBaseCurrency sets the base currency (default "USD").
func WithBaseCurrency(base string) Option {
	return func(s *Service) {
		s.baseCurrency = strings.ToUpper(strings.TrimSpace(base))
	}
}

// WithClock sets a custom clock function for deterministic time testing.
func WithClock(clock func() time.Time) Option {
	return func(s *Service) {
		s.clock = clock
	}
}

// WithInitialRates populates the in-memory rate cache.
func WithInitialRates(rates map[string]CachedRate) Option {
	return func(s *Service) {
		s.mu.Lock()
		defer s.mu.Unlock()
		for k, v := range rates {
			s.rates[strings.ToUpper(k)] = v
		}
	}
}

// NewService constructs a new production FXService.
func NewService(queries store.Querier, client RateFetcher, opts ...Option) *Service {
	s := &Service{
		queries:      queries,
		client:       client,
		baseCurrency: "USD",
		clock:        time.Now,
		rates:        make(map[string]CachedRate),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Convert converts a monetary amount from one currency to another using the latest cached or stored exchange rates.
func (s *Service) Convert(ctx context.Context, amount decimal.Decimal, from, to string) (decimal.Decimal, FXMetadata, error) {
	from = strings.ToUpper(strings.TrimSpace(from))
	to = strings.ToUpper(strings.TrimSpace(to))

	if from == "" || to == "" {
		return decimal.Zero, FXMetadata{}, ErrInvalidCurrency
	}
	if amount.IsNegative() {
		return decimal.Zero, FXMetadata{}, ErrNegativeAmount
	}

	if from == to {
		now := s.clock().UTC()
		return amount, FXMetadata{
			Rate:      decimal.NewFromInt(1),
			Source:    "identity",
			RateDate:  now.Format("2006-01-02"),
			FetchedAt: now,
		}, nil
	}

	rate, meta, err := s.GetRate(ctx, from, to)
	if err != nil {
		return decimal.Zero, FXMetadata{}, err
	}

	base := s.baseCurrency
	var converted decimal.Decimal

	s.mu.RLock()
	fromRate, fromOk := s.rates[from]
	toRate, toOk := s.rates[to]
	s.mu.RUnlock()

	switch {
	case from == base && toOk:
		converted = amount.Mul(toRate.Rate)
	case to == base && fromOk && !fromRate.Rate.IsZero():
		converted = amount.Div(fromRate.Rate)
	case fromOk && toOk && !fromRate.Rate.IsZero():
		converted = amount.Mul(toRate.Rate).Div(fromRate.Rate)
	default:
		converted = amount.Mul(rate)
	}

	return converted, meta, nil
}

// GetRate returns the conversion exchange rate from one currency to another.
func (s *Service) GetRate(ctx context.Context, from, to string) (decimal.Decimal, FXMetadata, error) {
	from = strings.ToUpper(strings.TrimSpace(from))
	to = strings.ToUpper(strings.TrimSpace(to))

	if from == "" || to == "" {
		return decimal.Zero, FXMetadata{}, ErrInvalidCurrency
	}

	if from == to {
		now := s.clock().UTC()
		return decimal.NewFromInt(1), FXMetadata{
			Rate:      decimal.NewFromInt(1),
			Source:    "identity",
			RateDate:  now.Format("2006-01-02"),
			FetchedAt: now,
		}, nil
	}

	s.mu.RLock()
	rate, meta, found := s.resolveRateFromCache(from, to)
	s.mu.RUnlock()

	if found {
		return rate, meta, nil
	}

	// Lazy DB fallback if not found in memory cache
	if s.queries != nil {
		if dbRate, dbMeta, dbFound := s.resolveRateFromDB(ctx, from, to); dbFound {
			return dbRate, dbMeta, nil
		}
	}

	return decimal.Zero, FXMetadata{}, fmt.Errorf("%w: %s to %s", ErrRateNotFound, from, to)
}

func (s *Service) resolveRateFromCache(from, to string) (decimal.Decimal, FXMetadata, bool) {
	base := s.baseCurrency

	if from == base {
		if r, ok := s.rates[to]; ok {
			return r.Rate, FXMetadata(r), true
		}
	}

	if to == base {
		if r, ok := s.rates[from]; ok && !r.Rate.IsZero() {
			invRate := decimal.NewFromInt(1).Div(r.Rate)
			return invRate, FXMetadata{
				Rate:       invRate,
				Source:     r.Source,
				RateDate:   r.RateDate,
				FetchedAt:  r.FetchedAt,
				IsFallback: r.IsFallback,
			}, true
		}
	}

	fromRate, fromOk := s.rates[from]
	toRate, toOk := s.rates[to]
	if fromOk && toOk && !fromRate.Rate.IsZero() {
		crossRate := toRate.Rate.Div(fromRate.Rate)
		isFallback := fromRate.IsFallback || toRate.IsFallback
		return crossRate, FXMetadata{
			Rate:       crossRate,
			Source:     fromRate.Source,
			RateDate:   toRate.RateDate,
			FetchedAt:  toRate.FetchedAt,
			IsFallback: isFallback,
		}, true
	}

	return decimal.Zero, FXMetadata{}, false
}

func (s *Service) resolveRateFromDB(ctx context.Context, from, to string) (decimal.Decimal, FXMetadata, bool) {
	base := s.baseCurrency

	if from != base {
		s.mu.RLock()
		_, fromOk := s.rates[from]
		s.mu.RUnlock()
		if !fromOk {
			row, err := s.queries.GetLatestFXRate(ctx, store.GetLatestFXRateParams{
				BaseCurrency:   base,
				TargetCurrency: from,
			})
			if err == nil {
				if cached, cErr := mapStoreRowToCachedRate(row); cErr == nil && !cached.Rate.IsZero() {
					s.mu.Lock()
					s.rates[from] = cached
					s.mu.Unlock()
				}
			}
		}
	}

	if to != base {
		s.mu.RLock()
		_, toOk := s.rates[to]
		s.mu.RUnlock()
		if !toOk {
			row, err := s.queries.GetLatestFXRate(ctx, store.GetLatestFXRateParams{
				BaseCurrency:   base,
				TargetCurrency: to,
			})
			if err == nil {
				if cached, cErr := mapStoreRowToCachedRate(row); cErr == nil && !cached.Rate.IsZero() {
					s.mu.Lock()
					s.rates[to] = cached
					s.mu.Unlock()
				}
			}
		}
	}

	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.resolveRateFromCache(from, to)
}

func mapStoreRowToCachedRate(row store.FxRate) (CachedRate, error) {
	decRate, err := store.NumericToDecimal(row.Rate)
	if err != nil {
		return CachedRate{}, err
	}
	var rateDateStr string
	if row.RateDate.Valid {
		rateDateStr = row.RateDate.Time.Format("2006-01-02")
	}
	var fetchedAtTime time.Time
	if row.FetchedAt.Valid {
		fetchedAtTime = row.FetchedAt.Time
	}
	return CachedRate{
		Rate:       decRate,
		Source:     row.Source,
		RateDate:   rateDateStr,
		FetchedAt:  fetchedAtTime,
		IsFallback: true,
	}, nil
}

// RefreshRates synchronizes exchange rates from Frankfurter and persists them to PostgreSQL.
// If the upstream API call fails, it falls back to the latest recorded rates in PostgreSQL.
func (s *Service) RefreshRates(ctx context.Context) error {
	now := s.clock().UTC()

	if s.client != nil {
		resp, err := s.client.FetchLatestRates(ctx, s.baseCurrency)
		if err == nil && resp != nil && len(resp.Rates) > 0 {
			s.mu.Lock()
			s.rates[s.baseCurrency] = CachedRate{
				Rate:       decimal.NewFromInt(1),
				Source:     "base",
				RateDate:   resp.Date,
				FetchedAt:  now,
				IsFallback: false,
			}
			for code, rate := range resp.Rates {
				s.rates[strings.ToUpper(code)] = CachedRate{
					Rate:       rate,
					Source:     "frankfurter",
					RateDate:   resp.Date,
					FetchedAt:  now,
					IsFallback: false,
				}
			}
			s.mu.Unlock()

			if s.queries != nil {
				rateDate, dErr := store.DateFromString(resp.Date)
				if dErr != nil {
					rateDate = store.DateFromTime(now)
				}
				fetchedAt := store.TimestamptzFromTime(now)

				for code, rate := range resp.Rates {
					numRate, nErr := store.DecimalToNumeric(rate)
					if nErr != nil {
						continue
					}
					_, upsertErr := s.queries.UpsertFXRate(ctx, store.UpsertFXRateParams{
						BaseCurrency:   s.baseCurrency,
						TargetCurrency: strings.ToUpper(code),
						Rate:           numRate,
						Source:         "frankfurter",
						RateDate:       rateDate,
						FetchedAt:      fetchedAt,
					})
					if upsertErr != nil {
						slog.WarnContext(ctx, "failed to persist fx rate to database", "target", code, "error", upsertErr)
					}
				}
			}

			return nil
		}

		slog.WarnContext(ctx, "failed to fetch latest fx rates from upstream, attempting database fallback", "error", err)
	}

	// Fallback to PostgreSQL
	if s.queries != nil {
		dbRates, err := s.queries.ListLatestFXRates(ctx, s.baseCurrency)
		if err == nil && len(dbRates) > 0 {
			s.mu.Lock()
			s.rates[s.baseCurrency] = CachedRate{
				Rate:       decimal.NewFromInt(1),
				Source:     "base",
				RateDate:   now.Format("2006-01-02"),
				FetchedAt:  now,
				IsFallback: true,
			}
			for _, row := range dbRates {
				if cached, cErr := mapStoreRowToCachedRate(row); cErr == nil && !cached.Rate.IsZero() {
					s.rates[strings.ToUpper(row.TargetCurrency)] = cached
				}
			}
			s.mu.Unlock()
			return nil
		}
	}

	return ErrUpstreamUnavailable
}

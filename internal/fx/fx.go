package fx

import (
	"context"
	"time"

	"github.com/shopspring/decimal"
)

// FXService converts monetary amounts between currencies.
type FXService interface {
	// Convert converts an amount from one currency to another.
	// Returns the converted amount, exchange rate metadata, and any error.
	Convert(ctx context.Context, amount decimal.Decimal, from, to string) (decimal.Decimal, FXMetadata, error)
	// GetRate returns the exchange rate from one currency to another along with metadata.
	GetRate(ctx context.Context, from, to string) (decimal.Decimal, FXMetadata, error)
	// RefreshRates syncs latest exchange rates from external upstream provider and updates storage/cache.
	RefreshRates(ctx context.Context) error
}

// FXMetadata contains provenance details for an exchange rate.
type FXMetadata struct {
	Rate       decimal.Decimal `json:"rate"`
	Source     string          `json:"source"`
	RateDate   string          `json:"rate_date"`
	FetchedAt  time.Time       `json:"fetched_at"`
	IsFallback bool            `json:"is_fallback,omitempty"`
}

// NoOpFXService is an identity FX service that returns amounts unchanged.
// It only supports same-currency conversions (e.g. USD → USD).
// For cross-currency requests, it returns ErrCurrencyConversionUnsupported.
type NoOpFXService struct{}

// NewNoOpFXService creates a new NoOpFXService.
func NewNoOpFXService() *NoOpFXService {
	return &NoOpFXService{}
}

// Convert returns the amount unchanged if from == to. Otherwise it returns an error.
func (s *NoOpFXService) Convert(ctx context.Context, amount decimal.Decimal, from, to string) (decimal.Decimal, FXMetadata, error) {
	if from == to {
		now := time.Now().UTC()
		return amount, FXMetadata{
			Rate:      decimal.NewFromInt(1),
			Source:    "identity",
			RateDate:  now.Format("2006-01-02"),
			FetchedAt: now,
		}, nil
	}
	return decimal.Zero, FXMetadata{}, ErrCurrencyConversionUnsupported
}

// GetRate returns 1 if from == to. Otherwise it returns an error.
func (s *NoOpFXService) GetRate(ctx context.Context, from, to string) (decimal.Decimal, FXMetadata, error) {
	if from == to {
		now := time.Now().UTC()
		return decimal.NewFromInt(1), FXMetadata{
			Rate:      decimal.NewFromInt(1),
			Source:    "identity",
			RateDate:  now.Format("2006-01-02"),
			FetchedAt: now,
		}, nil
	}
	return decimal.Zero, FXMetadata{}, ErrCurrencyConversionUnsupported
}

// RefreshRates is a no-op for NoOpFXService.
func (s *NoOpFXService) RefreshRates(ctx context.Context) error {
	return nil
}

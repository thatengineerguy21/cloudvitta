package fx

import "github.com/shopspring/decimal"

// FXService converts monetary amounts between currencies.
// Stage 1.6 uses the no-op identity implementation (NoOpFXService).
// A concrete implementation backed by a real FX data source is deferred to stage 3.
type FXService interface {
	// Convert converts an amount from one currency to another.
	// Returns the converted amount and the exchange rate used.
	Convert(amount decimal.Decimal, from, to string) (converted decimal.Decimal, rate decimal.Decimal, err error)
}

// NoOpFXService is an identity FX service that returns amounts unchanged.
// It only supports same-currency "conversions" (USD → USD).
// For cross-currency requests, it returns an error.
type NoOpFXService struct{}

// NewNoOpFXService creates a new NoOpFXService.
func NewNoOpFXService() *NoOpFXService {
	return &NoOpFXService{}
}

// Convert returns the amount unchanged if from == to. Otherwise it returns an error.
func (s *NoOpFXService) Convert(amount decimal.Decimal, from, to string) (decimal.Decimal, decimal.Decimal, error) {
	if from == to {
		return amount, decimal.NewFromInt(1), nil
	}
	return decimal.Zero, decimal.Zero, ErrCurrencyConversionUnsupported
}

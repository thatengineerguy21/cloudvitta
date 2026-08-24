package fx

import "errors"

var (
	// ErrCurrencyConversionUnsupported is returned when a currency conversion is requested that cannot be satisfied.
	ErrCurrencyConversionUnsupported = errors.New("fx: currency conversion not supported")
	// ErrRateNotFound is returned when an exchange rate is not found in cache or persistent store.
	ErrRateNotFound = errors.New("fx: exchange rate not found")
	// ErrInvalidCurrency is returned when a currency code is empty or invalid.
	ErrInvalidCurrency = errors.New("fx: invalid currency code")
	// ErrNegativeAmount is returned when attempting to convert a negative monetary amount.
	ErrNegativeAmount = errors.New("fx: amount cannot be negative")
	// ErrUpstreamUnavailable is returned when upstream FX providers and fallback storage are unreachable.
	ErrUpstreamUnavailable = errors.New("fx: upstream FX provider unavailable")
)

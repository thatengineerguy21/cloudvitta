package fx

import "errors"

// ErrCurrencyConversionUnsupported is returned when a cross-currency conversion is requested
// but the FX service does not support it (e.g. the no-op implementation in stage 1.6).
var ErrCurrencyConversionUnsupported = errors.New("fx: currency conversion not yet supported")

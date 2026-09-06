package rest

import "strings"

// DefaultUningestedWarnings returns the static warnings for providers whose ingestion is currently unavailable.
func DefaultUningestedWarnings() []ProviderWarning {
	return []ProviderWarning{
		{Provider: "oracle", Code: "not_yet_ingested", Message: "Oracle OCI ingestion currently unavailable"},
		{Provider: "ibm", Code: "not_yet_ingested", Message: "IBM Cloud ingestion currently unavailable"},
		{Provider: "alibaba", Code: "not_yet_ingested", Message: "Alibaba Cloud ingestion currently unavailable"},
		{Provider: "digitalocean", Code: "not_yet_ingested", Message: "DigitalOcean ingestion currently unavailable"},
	}
}

// DefaultStage3Warnings is a backward-compatible alias for DefaultUningestedWarnings.
func DefaultStage3Warnings() []ProviderWarning {
	return DefaultUningestedWarnings()
}

// NormalizeCurrencyAndWarnings extracts and normalizes the currency parameter, returning the raw requested currency,
// active pricing currency (always "USD"), and initial warnings slice containing standard uningested provider notices and
// any unsupported currency warning.
func NormalizeCurrencyAndWarnings(rawCurrency string) (reqCurrency string, effectiveCurrency string, warnings []ProviderWarning) {
	warnings = DefaultUningestedWarnings()
	reqCurrency = strings.TrimSpace(rawCurrency)
	if reqCurrency == "" {
		reqCurrency = "USD"
	}
	if reqCurrency != "USD" {
		warnings = append(warnings, ProviderWarning{
			Provider: "system",
			Code:     "non_usd_currency_unsupported",
			Message:  "Currency conversion is not yet supported. Prices are returned in USD.",
		})
	}
	return reqCurrency, "USD", warnings
}

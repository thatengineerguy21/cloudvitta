package rest

import "strings"

// DefaultStage3Warnings returns the static warnings for providers scheduled for ingestion in Stage 3.
func DefaultStage3Warnings() []ProviderWarning {
	return []ProviderWarning{
		{Provider: "oracle", Code: "not_yet_ingested", Message: "Oracle OCI ingestion lands in stage 3."},
		{Provider: "ibm", Code: "not_yet_ingested", Message: "IBM Cloud ingestion lands in stage 3."},
		{Provider: "alibaba", Code: "not_yet_ingested", Message: "Alibaba Cloud ingestion lands in stage 3."},
		{Provider: "digitalocean", Code: "not_yet_ingested", Message: "DigitalOcean ingestion lands in stage 3."},
	}
}

// NormalizeCurrencyAndWarnings extracts and normalizes the currency parameter, returning the raw requested currency,
// active pricing currency (always "USD"), and initial warnings slice containing standard stage 3 notices and
// any unsupported currency warning.
func NormalizeCurrencyAndWarnings(rawCurrency string) (reqCurrency string, effectiveCurrency string, warnings []ProviderWarning) {
	warnings = DefaultStage3Warnings()
	reqCurrency = strings.TrimSpace(rawCurrency)
	if reqCurrency == "" {
		reqCurrency = "USD"
	}
	if reqCurrency != "USD" {
		warnings = append(warnings, ProviderWarning{
			Provider: "system",
			Code:     "currency_conversion_not_yet_supported",
			Message:  "Currency conversion is not yet supported. Prices are returned in USD.",
		})
	}
	return reqCurrency, "USD", warnings
}

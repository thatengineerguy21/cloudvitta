package rest

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/shopspring/decimal"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/service"
	"github.com/thatengineerguy21/CloudVitta/internal/transport/rest/middleware"
)

// maxAllowedEgressGB represents the upper bound on single-request egress size (10 PB = 10,000,000 GB).
var maxAllowedEgressGB = decimal.NewFromInt(10_000_000)

// NetworkComparisonMeta represents meta info in the network comparison response envelope.
type NetworkComparisonMeta struct {
	APIVersion  string                 `json:"api_version"`
	GeneratedAt time.Time              `json:"generated_at"`
	Query       map[string]interface{} `json:"query"`
}

// NetworkResultEntry represents a single provider result item in the network comparison envelope.
type NetworkResultEntry struct {
	Provider          string                   `json:"provider"`
	SkuID             string                   `json:"sku_id"`
	MatchedSpec       domain.NetworkAttributes `json:"matched_spec"`
	MatchQuality      string                   `json:"match_quality"`
	MissingAttributes []string                 `json:"missing_attributes"`
	Price             PriceDetail              `json:"price"`
	MonthlyCostUSD    decimal.Decimal          `json:"monthly_cost_usd"`
	FetchedAt         time.Time                `json:"fetched_at"`
	Stale             bool                     `json:"stale"`
}

// NetworkComparisonResponse represents the full network comparison response envelope.
type NetworkComparisonResponse struct {
	Meta     NetworkComparisonMeta `json:"meta"`
	Results  []NetworkResultEntry  `json:"results"`
	Warnings []ProviderWarning     `json:"warnings"`
}

// NetworkHandler handles GET /api/v1/prices/network.
type NetworkHandler struct {
	pricingSvc *service.PricingService
}

// NewNetworkHandler constructs a new NetworkHandler.
func NewNetworkHandler(pricingSvc *service.PricingService) *NetworkHandler {
	return &NetworkHandler{
		pricingSvc: pricingSvc,
	}
}

// @Summary      Get network pricing comparison
// @Description  Returns normalized network / data transfer egress pricing across cloud providers (AWS, Azure, GCP) for requested specs.
// @Tags         prices
// @Produce      json
// @Param        egress_gb  query     string  false  "Requested network egress in GB (e.g. 500, max 10000000)"
// @Param        region     query     string  false  "Canonical region group (default: us-east)"
// @Param        currency   query     string  false  "Target currency code (default: USD)"
// @Success      200        {object}  NetworkComparisonResponse
// @Failure      400        {object}  middleware.RFC7807Error
// @Failure      429        {object}  middleware.RFC7807Error
// @Failure      500        {object}  middleware.RFC7807Error
// @Router       /api/v1/prices/network [get]
func (h *NetworkHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		middleware.WriteJSONError(w, r, http.StatusMethodNotAllowed, "https://cloudvitta.dev/errors/method-not-allowed", "Method Not Allowed", "Only GET method is supported.")
		return
	}

	q := r.URL.Query()

	// Parse parameters
	region := q.Get("region")
	if region == "" {
		region = "us-east"
	}

	warnings := []ProviderWarning{
		{Provider: "oracle", Code: "not_yet_ingested", Message: "Oracle OCI ingestion lands in stage 3."},
		{Provider: "ibm", Code: "not_yet_ingested", Message: "IBM Cloud ingestion lands in stage 3."},
		{Provider: "alibaba", Code: "not_yet_ingested", Message: "Alibaba Cloud ingestion lands in stage 3."},
		{Provider: "digitalocean", Code: "not_yet_ingested", Message: "DigitalOcean ingestion lands in stage 3."},
	}

	currency := q.Get("currency")
	reqCurrency := currency
	if reqCurrency == "" {
		reqCurrency = "USD"
	}
	if currency != "" && currency != "USD" {
		warnings = append(warnings, ProviderWarning{
			Provider: "system",
			Code:     "currency_conversion_not_yet_supported",
			Message:  "Currency conversion is not yet supported. Prices are returned in USD.",
		})
	}
	currency = "USD"

	egressGB := decimal.NewFromInt(1)
	hasExplicitEgress := false
	if egressStr := q.Get("egress_gb"); egressStr != "" {
		parsed, err := decimal.NewFromString(egressStr)
		if err != nil || parsed.LessThanOrEqual(decimal.Zero) || parsed.GreaterThan(maxAllowedEgressGB) {
			middleware.WriteJSONError(w, r, http.StatusBadRequest, "https://cloudvitta.dev/errors/invalid-parameter", "Invalid query parameter", "egress_gb must be a positive number no greater than 10000000")
			return
		}
		egressGB = parsed
		hasExplicitEgress = true
	}

	providers := []string{"aws", "azure", "gcp"}
	var results []NetworkResultEntry
	var providerErrors int

	for _, prov := range providers {
		obsList, err := h.pricingSvc.GetPrices(r.Context(), prov, "network", region)
		if err != nil {
			providerErrors++
			warnings = append(warnings, ProviderWarning{
				Provider: prov,
				Code:     "fetch_failed",
				Message:  err.Error(),
			})
			continue
		}

		if len(obsList) == 0 {
			warnings = append(warnings, ProviderWarning{
				Provider: prov,
				Code:     "no_data_available",
				Message:  "No network pricing data available for this region.",
			})
			continue
		}

		for _, obs := range obsList {
			matchedSpec := obs.NetworkAttributes
			if hasExplicitEgress {
				f, _ := egressGB.Float64()
				matchedSpec.EgressGB = f
			}

			unit := obs.Unit
			if unit == "" {
				unit = "GB"
			}

			// In stage 1.5, matching thresholds are deferred to stage 1.6.
			// Honesty vocabulary: "not_yet_scored" accurately describes status without fake delta numbers.
			results = append(results, NetworkResultEntry{
				Provider:          obs.Provider,
				SkuID:             obs.SkuID,
				MatchedSpec:       matchedSpec,
				MatchQuality:      "not_yet_scored", // match scoring lands in 1.6
				MissingAttributes: []string{},
				Price: PriceDetail{
					Amount:   obs.PriceAmount,
					Unit:     unit,
					Currency: currency,
				},
				MonthlyCostUSD: obs.PriceAmount.Mul(egressGB),
				FetchedAt:      obs.FetchedAt,
				Stale:          false,
			})
		}
	}

	// If all providers errored and produced zero results, return a 500 error
	if len(results) == 0 && providerErrors == len(providers) {
		middleware.WriteJSONError(w, r, http.StatusInternalServerError, "https://cloudvitta.dev/errors/internal-error", "Network pricing unavailable", "All providers failed to retrieve pricing data")
		return
	}

	queryMeta := map[string]interface{}{
		"category": "network",
		"region":   region,
		"currency": reqCurrency,
	}
	if hasExplicitEgress {
		queryMeta["egress_gb"] = egressGB
	}

	resp := NetworkComparisonResponse{
		Meta: NetworkComparisonMeta{
			APIVersion:  "v1",
			GeneratedAt: time.Now().UTC(),
			Query:       queryMeta,
		},
		Results:  results,
		Warnings: warnings,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

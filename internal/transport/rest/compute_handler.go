package rest

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/shopspring/decimal"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/service"
	"github.com/thatengineerguy21/CloudVitta/internal/transport/rest/middleware"
)

// ComputeComparisonMeta represents meta info in the comparison response envelope.
type ComputeComparisonMeta struct {
	APIVersion  string                 `json:"api_version"`
	GeneratedAt time.Time              `json:"generated_at"`
	Query       map[string]interface{} `json:"query"`
}

// PriceDetail represents price details in a result entry.
type PriceDetail struct {
	Amount   decimal.Decimal `json:"amount"`
	Unit     string          `json:"unit"`
	Currency string          `json:"currency"`
}

// ComputeResultEntry represents a single provider result item in the comparison response envelope.
type ComputeResultEntry struct {
	Provider            string                   `json:"provider"`
	SkuID               string                   `json:"sku_id"`
	MatchedSpec         domain.ComputeAttributes `json:"matched_spec"`
	MatchQuality        string                   `json:"match_quality"`
	MatchDeltaPct       float64                  `json:"match_delta_pct"`
	MissingAttributes   []string                 `json:"missing_attributes"`
	Price               PriceDetail              `json:"price"`
	NormalizedHourlyUSD decimal.Decimal          `json:"normalized_hourly_usd"`
	FetchedAt           time.Time                `json:"fetched_at"`
	Stale               bool                     `json:"stale"`
}

// ProviderWarning represents an item in the warnings array.
type ProviderWarning struct {
	Provider string `json:"provider"`
	Code     string `json:"code"`
	Message  string `json:"message"`
}

// ComputeComparisonResponse represents the full comparison response envelope (§10.2).
type ComputeComparisonResponse struct {
	Meta     ComputeComparisonMeta `json:"meta"`
	Results  []ComputeResultEntry  `json:"results"`
	Warnings []ProviderWarning     `json:"warnings"`
}

// ComputeHandler handles GET /api/v1/prices/compute.
type ComputeHandler struct {
	pricingSvc *service.PricingService
}

// NewComputeHandler constructs a new ComputeHandler.
func NewComputeHandler(pricingSvc *service.PricingService) *ComputeHandler {
	return &ComputeHandler{
		pricingSvc: pricingSvc,
	}
}

// @Summary      Get compute pricing comparison
// @Description  Returns normalized compute pricing across cloud providers for requested specs.
// @Tags         prices
// @Produce      json
// @Param        vcpu           query     number  false  "Requested vCPU count (e.g. 4)"
// @Param        ram_gb         query     number  false  "Requested RAM in GB (e.g. 16)"
// @Param        family         query     string  false  "Instance family filter (e.g. t3, c5)"
// @Param        region         query     string  false  "Canonical region group (default: us-east)"
// @Param        currency       query     string  false  "Target currency code (default: USD)"
// @Param        strict_family  query     bool    false  "Strict instance family matching (default: true)"
// @Success      200            {object}  ComputeComparisonResponse
// @Failure      400            {object}  middleware.RFC7807Error
// @Failure      429            {object}  middleware.RFC7807Error
// @Failure      500            {object}  middleware.RFC7807Error
// @Router       /api/v1/prices/compute [get]
func (h *ComputeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

	reqCurrency, currency, warnings := NormalizeCurrencyAndWarnings(q.Get("currency"))

	strictFamily := true
	if strictStr := q.Get("strict_family"); strictStr != "" {
		if b, err := strconv.ParseBool(strictStr); err == nil {
			strictFamily = b
		}
	}

	family := q.Get("family")

	var reqVCPU float64
	if vcpuStr := q.Get("vcpu"); vcpuStr != "" {
		v, err := strconv.ParseFloat(vcpuStr, 64)
		if err != nil || v <= 0 {
			middleware.WriteJSONError(w, r, http.StatusBadRequest, "https://cloudvitta.dev/errors/invalid-parameter", "Invalid query parameter", "vcpu must be a positive number")
			return
		}
		reqVCPU = v
	}

	var reqRAMGB float64
	if ramStr := q.Get("ram_gb"); ramStr != "" {
		v, err := strconv.ParseFloat(ramStr, 64)
		if err != nil || v <= 0 {
			middleware.WriteJSONError(w, r, http.StatusBadRequest, "https://cloudvitta.dev/errors/invalid-parameter", "Invalid query parameter", "ram_gb must be a positive number")
			return
		}
		reqRAMGB = v
	}

	providers := service.SupportedProviders()
	results := make([]ComputeResultEntry, 0)
	var providerErrors int

	for _, prov := range providers {
		// Score & match results using the service layer matching engine
		target := service.MatchTarget{
			VCPU:         reqVCPU,
			RAMGB:        reqRAMGB,
			Family:       family,
			StrictFamily: strictFamily,
			Category:     "compute",
		}

		catResult, err := h.pricingSvc.MatchAndCalculate(r.Context(), prov, "compute", region, target)
		if err != nil {
			switch err {
			case service.ErrCategoryNotSupported:
				warnings = append(warnings, ProviderWarning{
					Provider: prov,
					Code:     "category_not_supported",
					Message:  "Compute category is not supported by " + prov,
				})
			case service.ErrNoMatchFound:
				warnings = append(warnings, ProviderWarning{
					Provider: prov,
					Code:     "no_match",
					Message:  "No compute SKU matched the requested spec within acceptable thresholds.",
				})
			default:
				providerErrors++
				warnings = append(warnings, ProviderWarning{
					Provider: prov,
					Code:     "fetch_failed",
					Message:  err.Error(),
				})
			}
			continue
		}

		if catResult == nil {
			warnings = append(warnings, ProviderWarning{
				Provider: prov,
				Code:     "no_data_available",
				Message:  "No compute pricing data available for this region.",
			})
			continue
		}

		obs := catResult.MatchResult.Observation
		results = append(results, ComputeResultEntry{
			Provider:          obs.Provider,
			SkuID:             obs.SkuID,
			MatchedSpec:       obs.Attributes,
			MatchQuality:      catResult.MatchResult.MatchQuality,
			MatchDeltaPct:     catResult.MatchResult.MatchDeltaPct,
			MissingAttributes: catResult.MatchResult.MissingAttributes,
			Price: PriceDetail{
				Amount:   obs.PriceAmount,
				Unit:     catResult.Unit,
				Currency: currency,
			},
			NormalizedHourlyUSD: catResult.HourlyCost,
			FetchedAt:           obs.FetchedAt,
			Stale:               catResult.Stale,
		})
	}

	// If all providers errored and produced zero results, return a 502 Bad Gateway error
	if len(results) == 0 && providerErrors == len(providers) {
		middleware.WriteJSONError(w, r, http.StatusBadGateway, "https://cloudvitta.dev/errors/provider-unavailable", "Provider Unavailable", "All providers failed to retrieve pricing data")
		return
	}

	queryMeta := map[string]interface{}{
		"category": "compute",
		"region":   region,
		"currency": reqCurrency,
	}
	if reqVCPU > 0 {
		queryMeta["vcpu"] = reqVCPU
	}
	if reqRAMGB > 0 {
		queryMeta["ram_gb"] = reqRAMGB
	}
	if family != "" {
		queryMeta["family"] = family
	}
	if q.Get("strict_family") != "" {
		queryMeta["strict_family"] = strictFamily
	}

	resp := ComputeComparisonResponse{
		Meta: ComputeComparisonMeta{
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

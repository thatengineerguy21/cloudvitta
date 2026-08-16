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

// maxAllowedSizeGB represents the upper bound on single-request storage size (1 PB).
var maxAllowedSizeGB = decimal.NewFromInt(1_000_000)

// StorageComparisonMeta represents meta info in the storage comparison response envelope.
type StorageComparisonMeta struct {
	APIVersion  string                 `json:"api_version"`
	GeneratedAt time.Time              `json:"generated_at"`
	Query       map[string]interface{} `json:"query"`
}

// StorageResultEntry represents a single provider result item in the storage comparison envelope.
type StorageResultEntry struct {
	Provider          string                   `json:"provider"`
	SkuID             string                   `json:"sku_id"`
	MatchedSpec       domain.StorageAttributes `json:"matched_spec"`
	MatchQuality      string                   `json:"match_quality"`
	MatchDeltaPct     float64                  `json:"match_delta_pct"`
	MissingAttributes []string                 `json:"missing_attributes"`
	Price             PriceDetail              `json:"price"`
	MonthlyCostUSD    decimal.Decimal          `json:"monthly_cost_usd"`
	FetchedAt         time.Time                `json:"fetched_at"`
	Stale             bool                     `json:"stale"`
}

// StorageComparisonResponse represents the full storage comparison response envelope.
type StorageComparisonResponse struct {
	Meta     StorageComparisonMeta `json:"meta"`
	Results  []StorageResultEntry  `json:"results"`
	Warnings []ProviderWarning     `json:"warnings"`
}

// StorageHandler handles GET /api/v1/prices/storage.
type StorageHandler struct {
	pricingSvc *service.PricingService
}

// NewStorageHandler constructs a new StorageHandler.
func NewStorageHandler(pricingSvc *service.PricingService) *StorageHandler {
	return &StorageHandler{
		pricingSvc: pricingSvc,
	}
}

// @Summary      Get storage pricing comparison
// @Description  Returns normalized storage pricing across cloud providers (AWS, Azure, GCP) for requested specs.
// @Tags         prices
// @Produce      json
// @Param        size_gb        query     string  false  "Requested storage size in GB (e.g. 500, max 1000000)"
// @Param        storage_class  query     string  false  "Requested canonical storage class (e.g. standard, infrequent_access, archive)"
// @Param        region         query     string  false  "Canonical region group (default: us-east)"
// @Param        currency       query     string  false  "Target currency code (default: USD)"
// @Success      200            {object}  StorageComparisonResponse
// @Failure      400            {object}  middleware.RFC7807Error
// @Failure      429            {object}  middleware.RFC7807Error
// @Failure      500            {object}  middleware.RFC7807Error
// @Router       /api/v1/prices/storage [get]
func (h *StorageHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

	sizeGB := decimal.NewFromInt(1)
	hasExplicitSize := false
	if sizeStr := q.Get("size_gb"); sizeStr != "" {
		parsed, err := decimal.NewFromString(sizeStr)
		if err != nil || parsed.LessThanOrEqual(decimal.Zero) || parsed.GreaterThan(maxAllowedSizeGB) {
			middleware.WriteJSONError(w, r, http.StatusBadRequest, "https://cloudvitta.dev/errors/invalid-parameter", "Invalid query parameter", "size_gb must be a positive number no greater than 1000000")
			return
		}
		sizeGB = parsed
		hasExplicitSize = true
	}

	storageClass := q.Get("storage_class")

	providers := []string{"aws", "azure", "gcp"}
	var results []StorageResultEntry
	var providerErrors int

	for _, prov := range providers {
		// Build match target from query parameters.
		sizeF, _ := sizeGB.Float64()
		target := service.MatchTarget{
			SizeGB:       sizeF,
			StorageClass: storageClass,
			Category:     "storage",
		}

		catResult, err := h.pricingSvc.MatchAndCalculate(r.Context(), prov, "storage", region, target)
		if err != nil {
			switch err {
			case service.ErrCategoryNotSupported:
				warnings = append(warnings, ProviderWarning{
					Provider: prov,
					Code:     "category_not_supported",
					Message:  "Storage category is not supported by " + prov,
				})
			case service.ErrNoMatchFound:
				warnings = append(warnings, ProviderWarning{
					Provider: prov,
					Code:     "no_match",
					Message:  "No storage SKU matched the requested spec within acceptable thresholds.",
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
				Message:  "No storage pricing data available for this region.",
			})
			continue
		}

		obs := catResult.MatchResult.Observation

		results = append(results, StorageResultEntry{
			Provider:          obs.Provider,
			SkuID:             obs.SkuID,
			MatchedSpec:       obs.StorageAttributes,
			MatchQuality:      catResult.MatchResult.MatchQuality,
			MatchDeltaPct:     catResult.MatchResult.MatchDeltaPct,
			MissingAttributes: catResult.MatchResult.MissingAttributes,
			Price: PriceDetail{
				Amount:   obs.PriceAmount,
				Unit:     catResult.Unit,
				Currency: currency,
			},
			MonthlyCostUSD: catResult.MonthlyCost,
			FetchedAt:      obs.FetchedAt,
			Stale:          false,
		})
	}

	// If all providers errored and produced zero results, return a 500 error
	if len(results) == 0 && providerErrors == len(providers) {
		middleware.WriteJSONError(w, r, http.StatusInternalServerError, "https://cloudvitta.dev/errors/internal-error", "Storage pricing unavailable", "All providers failed to retrieve pricing data")
		return
	}

	queryMeta := map[string]interface{}{
		"category": "storage",
		"region":   region,
		"currency": reqCurrency,
	}
	if hasExplicitSize {
		queryMeta["size_gb"] = sizeGB
	}
	if storageClass != "" {
		queryMeta["storage_class"] = storageClass
	}

	resp := StorageComparisonResponse{
		Meta: StorageComparisonMeta{
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

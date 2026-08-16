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

// CalculateRequestBody represents the JSON payload for the composite calculate endpoint.
type CalculateRequestBody struct {
	Region       string                    `json:"region"`
	Currency     string                    `json:"currency"`
	StrictFamily *bool                     `json:"strict_family,omitempty"`
	Compute      *domain.ComputeAttributes `json:"compute,omitempty"`
	Storage      *domain.StorageAttributes `json:"storage,omitempty"`
	Network      *domain.NetworkAttributes `json:"network,omitempty"`
}

// CalculateCategoryResult represents a single category result inside a provider.
type CalculateCategoryResult struct {
	SkuID               string          `json:"sku_id"`
	MatchQuality        string          `json:"match_quality"`
	MatchDeltaPct       float64         `json:"match_delta_pct"`
	MissingAttributes   []string        `json:"missing_attributes"`
	Stale               bool            `json:"stale"`
	NormalizedHourlyUSD decimal.Decimal `json:"normalized_hourly_usd"`
}

// CalculateProviderResult represents a provider's overall calculation result.
type CalculateProviderResult struct {
	Provider                        string                             `json:"provider"`
	Categories                      map[string]CalculateCategoryResult `json:"categories"`
	TotalNormalizedHourlyUSD        *decimal.Decimal                   `json:"total_normalized_hourly_usd,omitempty"`
	PartialTotalNormalizedHourlyUSD *decimal.Decimal                   `json:"partial_total_normalized_hourly_usd,omitempty"`
	Partial                         bool                               `json:"partial"`
}

// CalculateMeta represents metadata for the calculate response.
type CalculateMeta struct {
	APIVersion  string               `json:"api_version"`
	GeneratedAt time.Time            `json:"generated_at"`
	Request     CalculateRequestBody `json:"request"`
}

// CalculateResponse represents the full calculate response envelope.
type CalculateResponse struct {
	Meta     CalculateMeta             `json:"meta"`
	Results  []CalculateProviderResult `json:"results"`
	Warnings []ProviderWarning         `json:"warnings"`
}

// CalculateHandler handles POST /api/v1/calculate.
type CalculateHandler struct {
	pricingSvc *service.PricingService
}

// NewCalculateHandler constructs a new CalculateHandler.
func NewCalculateHandler(pricingSvc *service.PricingService) *CalculateHandler {
	return &CalculateHandler{
		pricingSvc: pricingSvc,
	}
}

// @Summary      Get composite pricing calculation
// @Description  Calculates composite pricing across cloud providers for requested compute, storage, and network specs.
// @Tags         calculator
// @Accept       json
// @Produce      json
// @Param        payload  body      CalculateRequestBody  true  "Composite workload request payload"
// @Success      200      {object}  CalculateResponse
// @Failure      400      {object}  middleware.RFC7807Error
// @Failure      429      {object}  middleware.RFC7807Error
// @Failure      500      {object}  middleware.RFC7807Error
// @Router       /api/v1/calculate [post]
func (h *CalculateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		middleware.WriteJSONError(w, r, http.StatusMethodNotAllowed, "https://cloudvitta.dev/errors/method-not-allowed", "Method Not Allowed", "Only POST method is supported.")
		return
	}

	var reqBody CalculateRequestBody
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		middleware.WriteJSONError(w, r, http.StatusBadRequest, "https://cloudvitta.dev/errors/invalid-payload", "Invalid Payload", "Request body must be valid JSON.")
		return
	}
	defer func() { _ = r.Body.Close() }()

	if reqBody.Compute == nil && reqBody.Storage == nil && reqBody.Network == nil {
		middleware.WriteJSONError(w, r, http.StatusBadRequest, "https://cloudvitta.dev/errors/missing-categories", "Missing Categories", "At least one of 'compute', 'storage', or 'network' must be specified.")
		return
	}

	strictFamily := true
	if reqBody.StrictFamily != nil {
		strictFamily = *reqBody.StrictFamily
	}
	region := reqBody.Region
	if region == "" {
		region = "us-east"
	}
	currency := reqBody.Currency
	if currency == "" {
		currency = "USD"
	}

	var warnings []ProviderWarning
	if currency != "USD" {
		warnings = append(warnings, ProviderWarning{
			Provider: "system",
			Code:     "currency_conversion_not_yet_supported",
			Message:  "Currency conversion is not yet supported. Prices are returned in USD.",
		})
	}

	svcReq := service.CalculateRequest{
		Region:       region,
		Currency:     "USD",
		StrictFamily: strictFamily,
		Compute:      reqBody.Compute,
		Storage:      reqBody.Storage,
		Network:      reqBody.Network,
	}

	svcRes, err := h.pricingSvc.Calculate(r.Context(), svcReq)
	if err != nil {
		if err == service.ErrProviderUnavailable {
			middleware.WriteJSONError(w, r, http.StatusInternalServerError, "https://cloudvitta.dev/errors/internal-error", "Pricing unavailable", "All providers failed to retrieve pricing data")
			return
		}
		middleware.WriteJSONError(w, r, http.StatusInternalServerError, "https://cloudvitta.dev/errors/internal-error", "Internal Error", err.Error())
		return
	}

	for _, w := range svcRes.Warnings {
		warnings = append(warnings, ProviderWarning{
			Provider: w.Provider,
			Code:     w.Code,
			Message:  w.Message,
		})
	}

	// Add static warnings for unsupported providers from stage 3
	warnings = append(warnings, []ProviderWarning{
		{Provider: "oracle", Code: "not_yet_ingested", Message: "Oracle OCI ingestion lands in stage 3."},
		{Provider: "ibm", Code: "not_yet_ingested", Message: "IBM Cloud ingestion lands in stage 3."},
		{Provider: "alibaba", Code: "not_yet_ingested", Message: "Alibaba Cloud ingestion lands in stage 3."},
		{Provider: "digitalocean", Code: "not_yet_ingested", Message: "DigitalOcean ingestion lands in stage 3."},
	}...)

	var mappedResults []CalculateProviderResult
	for _, pr := range svcRes.Results {
		mappedCats := make(map[string]CalculateCategoryResult)
		for k, c := range pr.Categories {
			mappedCats[k] = CalculateCategoryResult{
				SkuID:               c.SkuID,
				MatchQuality:        c.MatchQuality,
				MatchDeltaPct:       c.MatchDeltaPct,
				MissingAttributes:   c.MissingAttributes,
				Stale:               c.Stale,
				NormalizedHourlyUSD: c.NormalizedHourlyUSD,
			}
		}

		mappedResults = append(mappedResults, CalculateProviderResult{
			Provider:                        pr.Provider,
			Categories:                      mappedCats,
			TotalNormalizedHourlyUSD:        pr.TotalNormalizedHourlyUSD,
			PartialTotalNormalizedHourlyUSD: pr.PartialTotalNormalizedHourlyUSD,
			Partial:                         pr.Partial,
		})
	}

	resp := CalculateResponse{
		Meta: CalculateMeta{
			APIVersion:  "v1",
			GeneratedAt: time.Now().UTC(),
			Request:     reqBody, // echo back what we processed
		},
		Results:  mappedResults,
		Warnings: warnings,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

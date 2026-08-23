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
	if err := DecodeJSONBody(w, r, &reqBody); err != nil {
		middleware.WriteJSONError(w, r, http.StatusBadRequest, "https://cloudvitta.dev/errors/invalid-parameter", "Invalid Parameters", "Malformed JSON request body")
		return
	}

	if reqBody.Compute == nil && reqBody.Storage == nil && reqBody.Network == nil {
		middleware.WriteJSONError(w, r, http.StatusBadRequest, "https://cloudvitta.dev/errors/missing-categories", "Missing Categories", "At least one of 'compute', 'storage', or 'network' must be specified.")
		return
	}

	if reqBody.Compute != nil {
		if reqBody.Compute.VCPU <= 0 {
			middleware.WriteJSONError(w, r, http.StatusBadRequest, "https://cloudvitta.dev/errors/invalid-parameter", "Invalid Parameters", "compute.vcpu must be a positive number")
			return
		}
		if reqBody.Compute.RAMGB <= 0 {
			middleware.WriteJSONError(w, r, http.StatusBadRequest, "https://cloudvitta.dev/errors/invalid-parameter", "Invalid Parameters", "compute.ram_gb must be a positive number")
			return
		}
	}

	if reqBody.Storage != nil {
		if reqBody.Storage.SizeGB <= 0 {
			middleware.WriteJSONError(w, r, http.StatusBadRequest, "https://cloudvitta.dev/errors/invalid-parameter", "Invalid Parameters", "storage.size_gb must be a positive number")
			return
		}
		if reqBody.Storage.SizeGB > 1000000 {
			middleware.WriteJSONError(w, r, http.StatusBadRequest, "https://cloudvitta.dev/errors/invalid-parameter", "Invalid Parameters", "storage.size_gb exceeds maximum limit of 1,000,000 GB (1 PB)")
			return
		}
	}

	if reqBody.Network != nil {
		if reqBody.Network.EgressGB < 0 {
			middleware.WriteJSONError(w, r, http.StatusBadRequest, "https://cloudvitta.dev/errors/invalid-parameter", "Invalid Parameters", "network.egress_gb cannot be negative")
			return
		}
		if reqBody.Network.EgressGB > 10000000 {
			middleware.WriteJSONError(w, r, http.StatusBadRequest, "https://cloudvitta.dev/errors/invalid-parameter", "Invalid Parameters", "network.egress_gb exceeds maximum limit of 10,000,000 GB (10 PB)")
			return
		}
	}

	strictFamily := true
	if reqBody.StrictFamily != nil {
		strictFamily = *reqBody.StrictFamily
	}
	region := reqBody.Region
	if region == "" {
		region = "us-east"
	}
	_, currency, warnings := NormalizeCurrencyAndWarnings(reqBody.Currency)

	svcReq := service.CalculateRequest{
		Region:       region,
		Currency:     currency,
		StrictFamily: strictFamily,
		Compute:      reqBody.Compute,
		Storage:      reqBody.Storage,
		Network:      reqBody.Network,
	}

	svcRes, err := h.pricingSvc.Calculate(r.Context(), svcReq)
	if err != nil {
		status, errType, title := middleware.MapServiceError(err)
		detail := err.Error()
		if status == http.StatusInternalServerError {
			detail = "An internal server error occurred while calculating pricing"
		}
		middleware.WriteJSONError(w, r, status, errType, title, detail)
		return
	}

	for _, w := range svcRes.Warnings {
		warnings = append(warnings, ProviderWarning{
			Provider: w.Provider,
			Code:     w.Code,
			Message:  w.Message,
		})
	}

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

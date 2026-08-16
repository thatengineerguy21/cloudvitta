package rest

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/shopspring/decimal"
	"github.com/thatengineerguy21/CloudVitta/internal/service"
	"github.com/thatengineerguy21/CloudVitta/internal/transport/rest/middleware"
)

// CalculateComputeRequest represents compute parameters in the calculation request.
type CalculateComputeRequest struct {
	VCPU         *float64 `json:"vcpu,omitempty"`
	RAMGB        *float64 `json:"ram_gb,omitempty"`
	Family       string   `json:"family,omitempty"`
	StrictFamily *bool    `json:"strict_family,omitempty"`
}

// CalculateStorageRequest represents storage parameters in the calculation request.
type CalculateStorageRequest struct {
	SizeGB       *decimal.Decimal `json:"size_gb,omitempty"`
	StorageClass string           `json:"storage_class,omitempty"`
}

// CalculateNetworkRequest represents network parameters in the calculation request.
type CalculateNetworkRequest struct {
	EgressGB     *decimal.Decimal `json:"egress_gb,omitempty"`
	TransferType string           `json:"transfer_type,omitempty"`
}

// CalculateRequestBody represents the incoming JSON request payload for POST /api/v1/calculate.
type CalculateRequestBody struct {
	Region   string                   `json:"region,omitempty"`
	Currency string                   `json:"currency,omitempty"`
	Compute  *CalculateComputeRequest `json:"compute,omitempty"`
	Storage  *CalculateStorageRequest `json:"storage,omitempty"`
	Network  *CalculateNetworkRequest `json:"network,omitempty"`
}

// CalculateComparisonMeta represents meta info in the composite calculation response envelope.
type CalculateComparisonMeta struct {
	APIVersion  string    `json:"api_version"`
	GeneratedAt time.Time `json:"generated_at"`
}

// CalculateComparisonResponse represents the full composite calculation response envelope.
type CalculateComparisonResponse struct {
	Meta     CalculateComparisonMeta           `json:"meta"`
	Results  []service.CalculateProviderResult `json:"results"`
	Warnings []ProviderWarning                 `json:"warnings"`
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

// @Summary      Calculate composite workload pricing
// @Description  Calculates server-side composite normalized hourly pricing across cloud providers (AWS, Azure, GCP) for multi-category workloads (compute, storage, network).
// @Tags         prices
// @Accept       json
// @Produce      json
// @Param        request body     CalculateRequestBody true "Workload calculation request"
// @Success      200     {object} CalculateComparisonResponse
// @Failure      400     {object} middleware.RFC7807Error
// @Failure      405     {object} middleware.RFC7807Error
// @Failure      429     {object} middleware.RFC7807Error
// @Failure      500     {object} middleware.RFC7807Error
// @Router       /api/v1/calculate [post]
func (h *CalculateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		middleware.WriteJSONError(w, r, http.StatusMethodNotAllowed, "https://cloudvitta.dev/errors/method-not-allowed", "Method Not Allowed", "Only POST method is supported.")
		return
	}

	var reqBody CalculateRequestBody
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		middleware.WriteJSONError(w, r, http.StatusBadRequest, "https://cloudvitta.dev/errors/invalid-parameter", "Invalid request body", "Malformed JSON body: "+err.Error())
		return
	}

	if reqBody.Compute == nil && reqBody.Storage == nil && reqBody.Network == nil {
		middleware.WriteJSONError(w, r, http.StatusBadRequest, "https://cloudvitta.dev/errors/invalid-parameter", "Invalid request body", "At least one category (compute, storage, network) must be specified")
		return
	}

	// Validate compute bounds
	if reqBody.Compute != nil {
		if reqBody.Compute.VCPU != nil && *reqBody.Compute.VCPU <= 0 {
			middleware.WriteJSONError(w, r, http.StatusBadRequest, "https://cloudvitta.dev/errors/invalid-parameter", "Invalid query parameter", "vcpu must be a positive number")
			return
		}
		if reqBody.Compute.RAMGB != nil && *reqBody.Compute.RAMGB <= 0 {
			middleware.WriteJSONError(w, r, http.StatusBadRequest, "https://cloudvitta.dev/errors/invalid-parameter", "Invalid query parameter", "ram_gb must be a positive number")
			return
		}
	}

	// Validate storage bounds
	if reqBody.Storage != nil && reqBody.Storage.SizeGB != nil {
		if reqBody.Storage.SizeGB.LessThanOrEqual(decimal.Zero) || reqBody.Storage.SizeGB.GreaterThan(maxAllowedSizeGB) {
			middleware.WriteJSONError(w, r, http.StatusBadRequest, "https://cloudvitta.dev/errors/invalid-parameter", "Invalid query parameter", "size_gb must be a positive number no greater than 1000000")
			return
		}
	}

	// Validate network bounds
	if reqBody.Network != nil && reqBody.Network.EgressGB != nil {
		if reqBody.Network.EgressGB.LessThanOrEqual(decimal.Zero) || reqBody.Network.EgressGB.GreaterThan(maxAllowedEgressGB) {
			middleware.WriteJSONError(w, r, http.StatusBadRequest, "https://cloudvitta.dev/errors/invalid-parameter", "Invalid query parameter", "egress_gb must be a positive number no greater than 10000000")
			return
		}
	}

	// Map to service request
	serviceReq := service.CalculateRequest{
		Region:   reqBody.Region,
		Currency: reqBody.Currency,
	}

	if reqBody.Compute != nil {
		var vcpu, ramGB float64
		if reqBody.Compute.VCPU != nil {
			vcpu = *reqBody.Compute.VCPU
		}
		if reqBody.Compute.RAMGB != nil {
			ramGB = *reqBody.Compute.RAMGB
		}
		strictFamily := true
		if reqBody.Compute.StrictFamily != nil {
			strictFamily = *reqBody.Compute.StrictFamily
		}
		serviceReq.Compute = &service.CalculateComputeTarget{
			VCPU:         vcpu,
			RAMGB:        ramGB,
			Family:       reqBody.Compute.Family,
			StrictFamily: strictFamily,
		}
	}

	if reqBody.Storage != nil {
		sizeGB := decimal.NewFromInt(1)
		if reqBody.Storage.SizeGB != nil {
			sizeGB = *reqBody.Storage.SizeGB
		}
		serviceReq.Storage = &service.CalculateStorageTarget{
			SizeGB:       sizeGB,
			StorageClass: reqBody.Storage.StorageClass,
		}
	}

	if reqBody.Network != nil {
		egressGB := decimal.NewFromInt(1)
		if reqBody.Network.EgressGB != nil {
			egressGB = *reqBody.Network.EgressGB
		}
		serviceReq.Network = &service.CalculateNetworkTarget{
			EgressGB:     egressGB,
			TransferType: reqBody.Network.TransferType,
		}
	}

	calcResult, err := h.pricingSvc.Calculate(r.Context(), serviceReq)
	if err != nil {
		if errors.Is(err, service.ErrProviderUnavailable) {
			middleware.WriteJSONError(w, r, http.StatusInternalServerError, "https://cloudvitta.dev/errors/internal-error", "Pricing unavailable", "All providers failed to retrieve pricing data")
			return
		}
		middleware.WriteJSONError(w, r, http.StatusBadRequest, "https://cloudvitta.dev/errors/invalid-parameter", "Calculation failed", err.Error())
		return
	}

	// Map warnings
	var warnings []ProviderWarning
	for _, w := range calcResult.Warnings {
		warnings = append(warnings, ProviderWarning{
			Provider: w.Provider,
			Code:     w.Code,
			Message:  w.Message,
		})
	}

	results := calcResult.Results
	if results == nil {
		results = []service.CalculateProviderResult{}
	}

	resp := CalculateComparisonResponse{
		Meta: CalculateComparisonMeta{
			APIVersion:  "v1",
			GeneratedAt: time.Now().UTC(),
		},
		Results:  results,
		Warnings: warnings,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

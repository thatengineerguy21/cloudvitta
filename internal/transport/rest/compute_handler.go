package rest

import (
	"encoding/json"
	"math"
	"net/http"
	"strconv"
	"time"

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
	Amount   float64 `json:"amount"`
	Unit     string  `json:"unit"`
	Currency string  `json:"currency"`
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
	NormalizedHourlyUSD float64                  `json:"normalized_hourly_usd"`
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
// @Param        region         query     string  false  "Canonical region group (default: us-east)"
// @Param        currency       query     string  false  "Target currency code (default: USD)"
// @Param        strict_family  query     bool    false  "Strict instance family matching"
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

	currency := q.Get("currency")
	if currency == "" {
		currency = "USD"
	}

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

	// Fetch compute prices from PricingService (which uses cache-aside + singleflight)
	obsList, err := h.pricingSvc.GetComputePrices(r.Context(), "aws", "compute", region)
	if err != nil {
		middleware.WriteJSONError(w, r, http.StatusInternalServerError, "https://cloudvitta.dev/errors/internal-server-error", "Internal Server Error", err.Error())
		return
	}

	// Score & filter results
	var results []ComputeResultEntry
	for _, obs := range obsList {
		quality := "exact"
		deltaPct := 0.0

		if reqVCPU > 0 || reqRAMGB > 0 {
			var vcpuDelta float64
			if reqVCPU > 0 {
				vcpuDelta = math.Abs(obs.Attributes.VCPU-reqVCPU) / reqVCPU
			}

			var ramDelta float64
			if reqRAMGB > 0 {
				ramDelta = math.Abs(obs.Attributes.RAMGB-reqRAMGB) / reqRAMGB
			}

			deltaPct = (vcpuDelta + ramDelta) * 100.0
			if deltaPct == 0.0 {
				quality = "exact"
			} else if deltaPct <= 50.0 {
				quality = "close"
			} else {
				// Delta too high, skip candidate
				continue
			}
		}

		priceFloat, _ := obs.PriceAmount.Float64()
		missingAttrs := []string{}

		results = append(results, ComputeResultEntry{
			Provider:          obs.Provider,
			SkuID:             obs.SkuID,
			MatchedSpec:       obs.Attributes,
			MatchQuality:      quality,
			MatchDeltaPct:     math.Round(deltaPct*100) / 100,
			MissingAttributes: missingAttrs,
			Price: PriceDetail{
				Amount:   priceFloat,
				Unit:     "hour",
				Currency: currency,
			},
			NormalizedHourlyUSD: priceFloat,
			FetchedAt:           obs.FetchedAt,
			Stale:               false,
		})
	}

	// Build warnings array for Stage 0 (only AWS ingested)
	warnings := []ProviderWarning{
		{Provider: "azure", Code: "not_yet_ingested", Message: "Azure ingestion lands in stage 1."},
		{Provider: "gcp", Code: "not_yet_ingested", Message: "GCP ingestion lands in stage 1."},
		{Provider: "oracle", Code: "not_yet_ingested", Message: "Oracle OCI ingestion lands in stage 3."},
		{Provider: "ibm", Code: "not_yet_ingested", Message: "IBM Cloud ingestion lands in stage 3."},
		{Provider: "alibaba", Code: "not_yet_ingested", Message: "Alibaba Cloud ingestion lands in stage 3."},
		{Provider: "digitalocean", Code: "not_yet_ingested", Message: "DigitalOcean ingestion lands in stage 3."},
	}

	queryMeta := map[string]interface{}{
		"category": "compute",
		"region":   region,
		"currency": currency,
	}
	if reqVCPU > 0 {
		queryMeta["vcpu"] = reqVCPU
	}
	if reqRAMGB > 0 {
		queryMeta["ram_gb"] = reqRAMGB
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

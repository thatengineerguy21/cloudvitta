package rest

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/kubernetestieremap"
	"github.com/thatengineerguy21/CloudVitta/internal/service"
	"github.com/thatengineerguy21/CloudVitta/internal/transport/rest/middleware"
)

// KubernetesComparisonMeta represents meta information in the Kubernetes comparison response envelope.
type KubernetesComparisonMeta struct {
	APIVersion  string                 `json:"api_version"`
	GeneratedAt time.Time              `json:"generated_at"`
	Query       map[string]interface{} `json:"query"`
}

// KubernetesResultEntry represents a single provider result item in the Kubernetes comparison response envelope.
type KubernetesResultEntry struct {
	Provider            string                      `json:"provider"`
	SkuID               string                      `json:"sku_id"`
	MatchedSpec         domain.KubernetesAttributes `json:"matched_spec"`
	MatchQuality        string                      `json:"match_quality"`
	MatchDeltaPct       float64                     `json:"match_delta_pct"`
	MissingAttributes   []string                    `json:"missing_attributes"`
	Price               PriceDetail                 `json:"price"`
	NormalizedHourlyUSD decimal.Decimal             `json:"normalized_hourly_usd"`
	FetchedAt           time.Time                   `json:"fetched_at"`
	Stale               bool                        `json:"stale"`
}

// KubernetesComparisonResponse represents the full Kubernetes comparison response envelope.
type KubernetesComparisonResponse struct {
	Meta     KubernetesComparisonMeta `json:"meta"`
	Results  []KubernetesResultEntry  `json:"results"`
	Warnings []ProviderWarning        `json:"warnings"`
}

// KubernetesHandler handles GET /api/v1/prices/kubernetes.
type KubernetesHandler struct {
	pricingSvc *service.PricingService
}

// NewKubernetesHandler constructs a new KubernetesHandler.
func NewKubernetesHandler(pricingSvc *service.PricingService) *KubernetesHandler {
	return &KubernetesHandler{
		pricingSvc: pricingSvc,
	}
}

// @Summary      Get Kubernetes control-plane pricing comparison
// @Description  Returns normalized Kubernetes control-plane pricing across cloud providers (AWS EKS, Azure AKS, GCP GKE) for requested tier.
// @Tags         prices
// @Produce      json
// @Param        tier              query     string  false  "Requested Kubernetes cluster tier (free, standard, extended_support; default: standard)"
// @Param        cluster_topology  query     string  false  "GCP-specific cluster topology for credit application (zonal, regional, autopilot; default: zonal)"
// @Param        region            query     string  false  "Canonical region group (default: us-east)"
// @Param        currency          query     string  false  "Target currency code (default: USD)"
// @Success      200               {object}  KubernetesComparisonResponse
// @Failure      400               {object}  middleware.RFC7807Error
// @Failure      429               {object}  middleware.RFC7807Error
// @Failure      500               {object}  middleware.RFC7807Error
// @Router       /api/v1/prices/kubernetes [get]
func (h *KubernetesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		middleware.WriteJSONError(w, r, http.StatusMethodNotAllowed, "https://cloudvitta.dev/errors/method-not-allowed", "Method Not Allowed", "Only GET method is supported.")
		return
	}

	q := r.URL.Query()

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

	rawTier := strings.TrimSpace(q.Get("tier"))
	canonicalTier := kubernetestieremap.TierStandard
	if rawTier != "" {
		resolved, err := kubernetestieremap.ResolveCanonicalTier(rawTier)
		if err != nil || !kubernetestieremap.IsSupportedStageTier(resolved) {
			middleware.WriteJSONError(w, r, http.StatusBadRequest, "https://cloudvitta.dev/errors/invalid-parameter", "Invalid query parameter", "tier must be a supported Kubernetes tier (free, standard, extended_support)")
			return
		}
		canonicalTier = resolved
	}

	rawTopology := strings.ToLower(strings.TrimSpace(q.Get("cluster_topology")))
	clusterTopology := "zonal"
	if rawTopology != "" {
		switch rawTopology {
		case "zonal", "regional", "autopilot":
			clusterTopology = rawTopology
		default:
			middleware.WriteJSONError(w, r, http.StatusBadRequest, "https://cloudvitta.dev/errors/invalid-parameter", "Invalid query parameter", "cluster_topology must be one of: zonal, regional, autopilot")
			return
		}
	}

	target := service.MatchTarget{
		Category:        "kubernetes",
		KubernetesTier:  canonicalTier,
		ClusterTopology: clusterTopology,
	}

	compResult, err := h.pricingSvc.Compare(r.Context(), "kubernetes", region, target)
	if err != nil {
		if errors.Is(err, service.ErrProviderUnavailable) {
			middleware.WriteJSONError(w, r, http.StatusServiceUnavailable, "https://cloudvitta.dev/errors/provider-unavailable", "Provider Unavailable", "All providers are currently unavailable for this region.")
			return
		}
		middleware.WriteJSONError(w, r, http.StatusInternalServerError, "https://cloudvitta.dev/errors/internal", "Internal Server Error", "An error occurred while evaluating Kubernetes pricing comparison.")
		return
	}

	for _, warn := range compResult.Warnings {
		warnings = append(warnings, ProviderWarning{
			Provider: warn.Provider,
			Code:     warn.Code,
			Message:  warn.Message,
		})
	}

	results := make([]KubernetesResultEntry, 0, len(compResult.Results))
	for _, item := range compResult.Results {
		results = append(results, KubernetesResultEntry{
			Provider:          item.Provider,
			SkuID:             item.SkuID,
			MatchedSpec:       item.MatchedKubernetes,
			MatchQuality:      item.MatchQuality,
			MatchDeltaPct:     item.MatchDeltaPct,
			MissingAttributes: item.MissingAttributes,
			Price: PriceDetail{
				Amount:   item.PriceAmount,
				Currency: item.PriceCurrency,
				Unit:     item.Unit,
			},
			NormalizedHourlyUSD: item.HourlyCost,
			FetchedAt:           item.FetchedAt,
			Stale:               item.Stale,
		})
	}

	queryMeta := map[string]interface{}{
		"tier":             canonicalTier,
		"cluster_topology": clusterTopology,
		"region":           region,
		"currency":         reqCurrency,
	}

	resp := KubernetesComparisonResponse{
		Meta: KubernetesComparisonMeta{
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

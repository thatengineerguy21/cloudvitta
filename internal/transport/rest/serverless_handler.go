package rest

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/shopspring/decimal"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/service"
	"github.com/thatengineerguy21/CloudVitta/internal/transport/rest/middleware"
)

// ServerlessComparisonMeta represents meta information in the Serverless comparison response envelope.
type ServerlessComparisonMeta struct {
	APIVersion  string                 `json:"api_version"`
	GeneratedAt time.Time              `json:"generated_at"`
	Query       map[string]interface{} `json:"query"`
}

// ServerlessResultEntry represents a single provider result item in the Serverless comparison response envelope.
type ServerlessResultEntry struct {
	Provider             string                          `json:"provider"`
	SkuID                string                          `json:"sku_id"`
	MatchedSpec          domain.ServerlessRateAttributes `json:"matched_spec"`
	MatchQuality         string                          `json:"match_quality"`
	MatchDeltaPct        float64                         `json:"match_delta_pct"`
	MissingAttributes    []string                        `json:"missing_attributes"`
	Price                PriceDetail                     `json:"price"`
	NormalizedHourlyUSD  decimal.Decimal                 `json:"normalized_hourly_usd"`
	NormalizedMonthlyUSD decimal.Decimal                 `json:"normalized_monthly_usd"`
	FetchedAt            time.Time                       `json:"fetched_at"`
	Stale                bool                            `json:"stale"`
}

// ServerlessComparisonResponse represents the full Serverless comparison response envelope.
type ServerlessComparisonResponse struct {
	Meta     ServerlessComparisonMeta `json:"meta"`
	Results  []ServerlessResultEntry  `json:"results"`
	Warnings []ProviderWarning        `json:"warnings"`
}

// ServerlessHandler handles GET /api/v1/prices/serverless.
type ServerlessHandler struct {
	pricingSvc *service.PricingService
}

// NewServerlessHandler constructs a new ServerlessHandler.
func NewServerlessHandler(pricingSvc *service.PricingService) *ServerlessHandler {
	return &ServerlessHandler{
		pricingSvc: pricingSvc,
	}
}

// @Summary      Get Serverless compute pricing comparison
// @Description  Returns normalized Serverless compute (FaaS) pricing across cloud providers (AWS Lambda, Azure Functions, GCP Cloud Functions) for requested workload.
// @Tags         prices
// @Produce      json
// @Param        architecture          query     string  false  "Requested CPU architecture (x86_64, arm64; default: x86_64)"
// @Param        tier                  query     string  false  "Requested serverless tier (consumption, flex_consumption, 1st_gen, 2nd_gen; default: consumption)"
// @Param        requests_per_month    query     number  false  "Monthly invocation requests (default: 1000000)"
// @Param        memory_mb             query     number  false  "Function allocated memory in MB (128 - 10240, default: 512)"
// @Param        execution_duration_ms query     number  false  "Average execution duration in ms (1 - 900000, default: 200)"
// @Param        region                query     string  false  "Canonical region group (default: us-east)"
// @Param        currency              query     string  false  "Target currency code (default: USD)"
// @Success      200                   {object}  ServerlessComparisonResponse
// @Failure      400                   {object}  middleware.RFC7807Error
// @Failure      429                   {object}  middleware.RFC7807Error
// @Failure      500                   {object}  middleware.RFC7807Error
// @Router       /api/v1/prices/serverless [get]
func (h *ServerlessHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		middleware.WriteJSONError(w, r, http.StatusMethodNotAllowed, "https://cloudvitta.dev/errors/method-not-allowed", "Method Not Allowed", "Only GET method is supported.")
		return
	}

	q := r.URL.Query()

	region := q.Get("region")
	if region == "" {
		region = "us-east"
	}

	reqCurrency, _, warnings := NormalizeCurrencyAndWarnings(q.Get("currency"))

	workload, err := service.ParseServerlessWorkload(
		q.Get("architecture"),
		q.Get("tier"),
		q.Get("requests_per_month"),
		q.Get("memory_mb"),
		q.Get("execution_duration_ms"),
	)
	if err != nil {
		middleware.WriteJSONError(w, r, http.StatusBadRequest, "https://cloudvitta.dev/errors/invalid-parameter", "Invalid query parameter", err.Error())
		return
	}

	target := service.MatchTarget{
		Category:           "serverless",
		ServerlessWorkload: workload,
	}

	compResult, err := h.pricingSvc.Compare(r.Context(), "serverless", region, target)
	if err != nil {
		if errors.Is(err, service.ErrProviderUnavailable) {
			middleware.WriteJSONError(w, r, http.StatusServiceUnavailable, "https://cloudvitta.dev/errors/provider-unavailable", "Provider Unavailable", "All providers are currently unavailable for this region.")
			return
		}
		middleware.WriteJSONError(w, r, http.StatusInternalServerError, "https://cloudvitta.dev/errors/internal", "Internal Server Error", "An error occurred while evaluating Serverless pricing comparison.")
		return
	}

	for _, warn := range compResult.Warnings {
		warnings = append(warnings, ProviderWarning{
			Provider: warn.Provider,
			Code:     warn.Code,
			Message:  warn.Message,
		})
	}

	results := make([]ServerlessResultEntry, 0, len(compResult.Results))
	for _, item := range compResult.Results {
		results = append(results, ServerlessResultEntry{
			Provider:          item.Provider,
			SkuID:             item.SkuID,
			MatchedSpec:       item.MatchedServerless,
			MatchQuality:      item.MatchQuality,
			MatchDeltaPct:     item.MatchDeltaPct,
			MissingAttributes: item.MissingAttributes,
			Price: PriceDetail{
				Amount:   item.PriceAmount,
				Currency: item.PriceCurrency,
				Unit:     item.Unit,
			},
			NormalizedHourlyUSD:  item.HourlyCost,
			NormalizedMonthlyUSD: item.MonthlyCost,
			FetchedAt:            item.FetchedAt,
			Stale:                item.Stale,
		})
	}

	queryMeta := map[string]interface{}{
		"architecture":          workload.Architecture,
		"tier":                  workload.Tier,
		"requests_per_month":    workload.RequestsPerMonth,
		"memory_mb":             workload.MemoryMB,
		"execution_duration_ms": workload.ExecutionDurationMS,
		"region":                region,
		"currency":              reqCurrency,
	}

	resp := ServerlessComparisonResponse{
		Meta: ServerlessComparisonMeta{
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

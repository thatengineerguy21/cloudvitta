package rest

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/nosqldatamodelmap"
	"github.com/thatengineerguy21/CloudVitta/internal/service"
	"github.com/thatengineerguy21/CloudVitta/internal/transport/rest/middleware"
)

// Bounds for NoSQL query parameters.
var (
	maxAllowedNoSQLStorageGB  = 1_000_000.0
	maxAllowedNoSQLThroughput = 10_000_000.0
)

// DatabaseNoSQLComparisonMeta represents meta information in the NoSQL database comparison response envelope.
type DatabaseNoSQLComparisonMeta struct {
	APIVersion  string                 `json:"api_version"`
	GeneratedAt time.Time              `json:"generated_at"`
	Query       map[string]interface{} `json:"query"`
}

// DatabaseNoSQLResultEntry represents a single provider result item in the NoSQL database comparison response envelope.
type DatabaseNoSQLResultEntry struct {
	Provider            string                         `json:"provider"`
	SkuID               string                         `json:"sku_id"`
	MatchedSpec         domain.DatabaseNoSQLAttributes `json:"matched_spec"`
	MatchQuality        string                         `json:"match_quality"`
	MatchDeltaPct       float64                        `json:"match_delta_pct"`
	MissingAttributes   []string                       `json:"missing_attributes"`
	Price               PriceDetail                    `json:"price"`
	NormalizedHourlyUSD decimal.Decimal                `json:"normalized_hourly_usd"`
	FetchedAt           time.Time                      `json:"fetched_at"`
	Stale               bool                           `json:"stale"`
}

// DatabaseNoSQLComparisonResponse represents the full NoSQL database comparison response envelope.
type DatabaseNoSQLComparisonResponse struct {
	Meta     DatabaseNoSQLComparisonMeta `json:"meta"`
	Results  []DatabaseNoSQLResultEntry  `json:"results"`
	Warnings []ProviderWarning           `json:"warnings"`
}

// DatabaseNoSQLHandler handles GET /api/v1/prices/database-nosql.
type DatabaseNoSQLHandler struct {
	pricingSvc *service.PricingService
}

// NewDatabaseNoSQLHandler constructs a new DatabaseNoSQLHandler.
func NewDatabaseNoSQLHandler(pricingSvc *service.PricingService) *DatabaseNoSQLHandler {
	return &DatabaseNoSQLHandler{
		pricingSvc: pricingSvc,
	}
}

// @Summary      Get NoSQL database pricing comparison
// @Description  Returns normalized NoSQL database pricing across cloud providers (AWS DynamoDB, Azure Cosmos DB, GCP Firestore) for requested specs.
// @Tags         prices
// @Produce      json
// @Param        data_model      query     string  false  "Requested NoSQL data model (e.g. document, key_value, wide_column, graph)"
// @Param        pricing_mode    query     string  false  "Requested pricing mode (provisioned, on_demand, serverless; default: provisioned)"
// @Param        read_units      query     number  false  "Requested reads per second (e.g. 100)"
// @Param        write_units     query     number  false  "Requested writes per second (e.g. 20)"
// @Param        storage_gb      query     number  false  "Requested storage in GB (e.g. 50)"
// @Param        storage_class   query     string  false  "Storage class preference (e.g. standard, infrequent_access, analytical)"
// @Param        multi_region    query     bool    false  "High Availability / Multi-Region replication (default: false)"
// @Param        region          query     string  false  "Canonical region group (default: us-east)"
// @Param        currency        query     string  false  "Target currency code (default: USD)"
// @Success      200             {object}  DatabaseNoSQLComparisonResponse
// @Failure      400             {object}  middleware.RFC7807Error
// @Failure      429             {object}  middleware.RFC7807Error
// @Failure      500             {object}  middleware.RFC7807Error
// @Router       /api/v1/prices/database-nosql [get]
func (h *DatabaseNoSQLHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		middleware.WriteJSONError(w, r, http.StatusMethodNotAllowed, "https://cloudvitta.dev/errors/method-not-allowed", "Method Not Allowed", "Only GET method is supported.")
		return
	}

	q := r.URL.Query()

	region := q.Get("region")
	if region == "" {
		region = "us-east"
	}

	reqCurrency, currency, warnings := NormalizeCurrencyAndWarnings(q.Get("currency"))

	rawModel := strings.TrimSpace(q.Get("data_model"))
	var canonicalModel string
	if rawModel != "" {
		canonical, err := nosqldatamodelmap.ResolveCanonicalDataModel(rawModel)
		if err != nil || !nosqldatamodelmap.IsSupportedStageDataModel(canonical) {
			middleware.WriteJSONError(w, r, http.StatusBadRequest, "https://cloudvitta.dev/errors/invalid-parameter", "Invalid query parameter", "data_model must be a supported NoSQL data model (document, key_value, wide_column, graph, multi_model)")
			return
		}
		canonicalModel = canonical
	}

	rawPricingMode := strings.ToLower(strings.TrimSpace(q.Get("pricing_mode")))
	if rawPricingMode == "" {
		rawPricingMode = "provisioned"
	}
	if rawPricingMode != "provisioned" && rawPricingMode != "on_demand" && rawPricingMode != "serverless" {
		middleware.WriteJSONError(w, r, http.StatusBadRequest, "https://cloudvitta.dev/errors/invalid-parameter", "Invalid query parameter", "pricing_mode must be provisioned, on_demand, or serverless")
		return
	}

	var reqReads float64
	if readsStr := q.Get("read_units"); readsStr != "" {
		v, err := strconv.ParseFloat(readsStr, 64)
		if err != nil || v < 0 || v > maxAllowedNoSQLThroughput {
			middleware.WriteJSONError(w, r, http.StatusBadRequest, "https://cloudvitta.dev/errors/invalid-parameter", "Invalid query parameter", "read_units must be a non-negative number up to 10000000")
			return
		}
		reqReads = v
	}

	var reqWrites float64
	if writesStr := q.Get("write_units"); writesStr != "" {
		v, err := strconv.ParseFloat(writesStr, 64)
		if err != nil || v < 0 || v > maxAllowedNoSQLThroughput {
			middleware.WriteJSONError(w, r, http.StatusBadRequest, "https://cloudvitta.dev/errors/invalid-parameter", "Invalid query parameter", "write_units must be a non-negative number up to 10000000")
			return
		}
		reqWrites = v
	}

	var reqStorageGB float64
	if storStr := q.Get("storage_gb"); storStr != "" {
		v, err := strconv.ParseFloat(storStr, 64)
		if err != nil || v < 0 || v > maxAllowedNoSQLStorageGB {
			middleware.WriteJSONError(w, r, http.StatusBadRequest, "https://cloudvitta.dev/errors/invalid-parameter", "Invalid query parameter", "storage_gb must be a non-negative number up to 1000000")
			return
		}
		reqStorageGB = v
	}

	storageClass := q.Get("storage_class")

	var multiRegion bool
	if mrStr := q.Get("multi_region"); mrStr != "" {
		if b, err := strconv.ParseBool(mrStr); err == nil {
			multiRegion = b
		}
	}

	target := service.MatchTarget{
		DataModel:        canonicalModel,
		PricingMode:      rawPricingMode,
		ReadUnits:        reqReads,
		WriteUnits:       reqWrites,
		NoSQLStorageGB:   reqStorageGB,
		StorageClass:     storageClass,
		NoSQLMultiRegion: multiRegion,
		Category:         "database_nosql",
	}

	compRes, err := h.pricingSvc.Compare(r.Context(), "database_nosql", region, target)
	if err != nil {
		if errors.Is(err, service.ErrProviderUnavailable) {
			middleware.WriteJSONError(w, r, http.StatusBadGateway, "https://cloudvitta.dev/errors/provider-unavailable", "Provider Unavailable", "All providers failed to retrieve pricing data")
			return
		}
		middleware.WriteJSONError(w, r, http.StatusInternalServerError, "https://cloudvitta.dev/errors/internal-error", "Internal Server Error", err.Error())
		return
	}

	var results []DatabaseNoSQLResultEntry
	for _, item := range compRes.Results {
		results = append(results, DatabaseNoSQLResultEntry{
			Provider:          item.Provider,
			SkuID:             item.SkuID,
			MatchedSpec:       item.MatchedNoSQL,
			MatchQuality:      item.MatchQuality,
			MatchDeltaPct:     item.MatchDeltaPct,
			MissingAttributes: item.MissingAttributes,
			Price: PriceDetail{
				Amount:   item.HourlyCost,
				Unit:     item.Unit,
				Currency: currency,
			},
			NormalizedHourlyUSD: item.HourlyCost,
			FetchedAt:           item.FetchedAt,
			Stale:               item.Stale,
		})
	}

	for _, wItem := range compRes.Warnings {
		warnings = append(warnings, ProviderWarning{
			Provider: wItem.Provider,
			Code:     wItem.Code,
			Message:  wItem.Message,
		})
	}

	queryMeta := map[string]interface{}{
		"category":     "database_nosql",
		"region":       region,
		"pricing_mode": rawPricingMode,
		"currency":     reqCurrency,
	}
	if rawModel != "" {
		queryMeta["data_model"] = canonicalModel
	}
	if reqReads > 0 {
		queryMeta["read_units"] = reqReads
	}
	if reqWrites > 0 {
		queryMeta["write_units"] = reqWrites
	}
	if reqStorageGB > 0 {
		queryMeta["storage_gb"] = reqStorageGB
	}
	if storageClass != "" {
		queryMeta["storage_class"] = storageClass
	}
	if q.Get("multi_region") != "" {
		queryMeta["multi_region"] = multiRegion
	}

	resp := DatabaseNoSQLComparisonResponse{
		Meta: DatabaseNoSQLComparisonMeta{
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

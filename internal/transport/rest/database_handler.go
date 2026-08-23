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
	"github.com/thatengineerguy21/CloudVitta/internal/matching/databaseenginemap"
	"github.com/thatengineerguy21/CloudVitta/internal/service"
	"github.com/thatengineerguy21/CloudVitta/internal/transport/rest/middleware"
)

// maxAllowedDatabaseStorageGB represents the upper bound on single-request database storage size (1 PB).
var maxAllowedDatabaseStorageGB = 1_000_000.0

// DatabaseComparisonMeta represents meta info in the database comparison response envelope.
type DatabaseComparisonMeta struct {
	APIVersion  string                 `json:"api_version"`
	GeneratedAt time.Time              `json:"generated_at"`
	Query       map[string]interface{} `json:"query"`
}

// DatabaseResultEntry represents a single provider result item in the database comparison response envelope.
type DatabaseResultEntry struct {
	Provider            string                         `json:"provider"`
	SkuID               string                         `json:"sku_id"`
	MatchedSpec         domain.DatabaseRDBMSAttributes `json:"matched_spec"`
	MatchQuality        string                         `json:"match_quality"`
	MatchDeltaPct       float64                        `json:"match_delta_pct"`
	MissingAttributes   []string                       `json:"missing_attributes"`
	Price               PriceDetail                    `json:"price"`
	NormalizedHourlyUSD decimal.Decimal                `json:"normalized_hourly_usd"`
	FetchedAt           time.Time                      `json:"fetched_at"`
	Stale               bool                           `json:"stale"`
}

// DatabaseComparisonResponse represents the full database comparison response envelope.
type DatabaseComparisonResponse struct {
	Meta     DatabaseComparisonMeta `json:"meta"`
	Results  []DatabaseResultEntry  `json:"results"`
	Warnings []ProviderWarning      `json:"warnings"`
}

// DatabaseHandler handles GET /api/v1/prices/database.
type DatabaseHandler struct {
	pricingSvc *service.PricingService
}

// NewDatabaseHandler constructs a new DatabaseHandler.
func NewDatabaseHandler(pricingSvc *service.PricingService) *DatabaseHandler {
	return &DatabaseHandler{
		pricingSvc: pricingSvc,
	}
}

// @Summary      Get relational database pricing comparison
// @Description  Returns normalized relational database pricing across cloud providers (AWS, Azure, GCP) for requested specs.
// @Tags         prices
// @Produce      json
// @Param        engine          query     string  false  "Requested database engine (e.g. postgresql, mysql, sqlserver)"
// @Param        vcpu            query     number  false  "Requested vCPU count (e.g. 4)"
// @Param        ram_gb          query     number  false  "Requested RAM in GB (e.g. 16)"
// @Param        storage_gb      query     number  false  "Requested storage in GB (e.g. 100)"
// @Param        iops            query     int     false  "Requested provisioned IOPS (e.g. 3000)"
// @Param        multi_az        query     bool    false  "High Availability / Multi-AZ deployment (default: false)"
// @Param        storage_family  query     string  false  "Storage family preference (e.g. gp3, ssd, io1)"
// @Param        region          query     string  false  "Canonical region group (default: us-east)"
// @Param        currency        query     string  false  "Target currency code (default: USD)"
// @Success      200             {object}  DatabaseComparisonResponse
// @Failure      400             {object}  middleware.RFC7807Error
// @Failure      429             {object}  middleware.RFC7807Error
// @Failure      500             {object}  middleware.RFC7807Error
// @Router       /api/v1/prices/database [get]
func (h *DatabaseHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

	rawEngine := strings.TrimSpace(q.Get("engine"))
	var canonicalEngine string
	if rawEngine != "" {
		canonical, err := databaseenginemap.ResolveCanonicalEngine(rawEngine)
		if err != nil || !databaseenginemap.IsSupportedStageEngine(canonical) {
			middleware.WriteJSONError(w, r, http.StatusBadRequest, "https://cloudvitta.dev/errors/invalid-parameter", "Invalid query parameter", "engine must be a supported database engine (postgresql, mysql, sqlserver)")
			return
		}
		canonicalEngine = canonical
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

	var reqStorageGB float64
	if storStr := q.Get("storage_gb"); storStr != "" {
		v, err := strconv.ParseFloat(storStr, 64)
		if err != nil || v <= 0 || v > maxAllowedDatabaseStorageGB {
			middleware.WriteJSONError(w, r, http.StatusBadRequest, "https://cloudvitta.dev/errors/invalid-parameter", "Invalid query parameter", "storage_gb must be a positive number up to 1000000")
			return
		}
		reqStorageGB = v
	}

	var reqIOPS *int
	if iopsStr := q.Get("iops"); iopsStr != "" {
		v, err := strconv.Atoi(iopsStr)
		if err != nil || v <= 0 {
			middleware.WriteJSONError(w, r, http.StatusBadRequest, "https://cloudvitta.dev/errors/invalid-parameter", "Invalid query parameter", "iops must be a positive integer")
			return
		}
		reqIOPS = &v
	}

	var multiAZ bool
	if mazStr := q.Get("multi_az"); mazStr != "" {
		if b, err := strconv.ParseBool(mazStr); err == nil {
			multiAZ = b
		}
	}

	storageFamily := q.Get("storage_family")

	target := service.MatchTarget{
		Engine:            canonicalEngine,
		VCPU:              reqVCPU,
		RAMGB:             reqRAMGB,
		DatabaseStorageGB: reqStorageGB,
		DatabaseIOPS:      reqIOPS,
		MultiAZ:           multiAZ,
		StorageFamily:     storageFamily,
		Category:          "database_rdbms",
	}

	compRes, err := h.pricingSvc.Compare(r.Context(), "database_rdbms", region, target)
	if err != nil {
		if errors.Is(err, service.ErrProviderUnavailable) {
			middleware.WriteJSONError(w, r, http.StatusBadGateway, "https://cloudvitta.dev/errors/provider-unavailable", "Provider Unavailable", "All providers failed to retrieve pricing data")
			return
		}
		middleware.WriteJSONError(w, r, http.StatusInternalServerError, "https://cloudvitta.dev/errors/internal-error", "Internal Server Error", err.Error())
		return
	}

	var results []DatabaseResultEntry
	for _, item := range compRes.Results {
		results = append(results, DatabaseResultEntry{
			Provider:          item.Provider,
			SkuID:             item.SkuID,
			MatchedSpec:       item.MatchedDatabase,
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
		"category": "database_rdbms",
		"region":   region,
		"currency": reqCurrency,
	}
	if rawEngine != "" {
		queryMeta["engine"] = canonicalEngine
	}
	if reqVCPU > 0 {
		queryMeta["vcpu"] = reqVCPU
	}
	if reqRAMGB > 0 {
		queryMeta["ram_gb"] = reqRAMGB
	}
	if reqStorageGB > 0 {
		queryMeta["storage_gb"] = reqStorageGB
	}
	if reqIOPS != nil {
		queryMeta["iops"] = *reqIOPS
	}
	if q.Get("multi_az") != "" {
		queryMeta["multi_az"] = multiAZ
	}
	if storageFamily != "" {
		queryMeta["storage_family"] = storageFamily
	}

	resp := DatabaseComparisonResponse{
		Meta: DatabaseComparisonMeta{
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

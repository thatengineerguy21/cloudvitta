package rest

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/databaseenginemap"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/kubernetestieremap"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/nosqldatamodelmap"
	"github.com/thatengineerguy21/CloudVitta/internal/service"
	"github.com/thatengineerguy21/CloudVitta/internal/transport/rest/middleware"
)

// ServerlessWorkloadPayload represents user-specified workload parameters for serverless compute.
type ServerlessWorkloadPayload struct {
	Architecture        string           `json:"architecture,omitempty"`
	Tier                string           `json:"tier,omitempty"`
	ExecutionDurationMS *decimal.Decimal `json:"execution_duration_ms,omitempty"`
	MemoryMB            *decimal.Decimal `json:"memory_mb,omitempty"`
	RequestsPerMonth    *decimal.Decimal `json:"requests_per_month,omitempty"`
}

// CalculateRequestBody represents the JSON payload for the composite calculate endpoint.
type CalculateRequestBody struct {
	Region        string                          `json:"region"`
	Currency      string                          `json:"currency"`
	StrictFamily  *bool                           `json:"strict_family,omitempty"`
	Compute       *domain.ComputeAttributes       `json:"compute,omitempty"`
	Storage       *domain.StorageAttributes       `json:"storage,omitempty"`
	Network       *domain.NetworkAttributes       `json:"network,omitempty"`
	DatabaseRDBMS *domain.DatabaseRDBMSAttributes `json:"database_rdbms,omitempty"`
	Database      *domain.DatabaseRDBMSAttributes `json:"database,omitempty"` // alias for database_rdbms
	DatabaseNoSQL *domain.DatabaseNoSQLAttributes `json:"database_nosql,omitempty"`
	Kubernetes    *domain.KubernetesAttributes    `json:"kubernetes,omitempty"`
	Serverless    *ServerlessWorkloadPayload      `json:"serverless,omitempty"`
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
// @Description  Calculates composite pricing across cloud providers for requested compute, storage, network, database, NoSQL, Kubernetes, and serverless specs.
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

	// Alias-conflict validation: database vs database_rdbms
	dbReq, err := service.ResolveAliasedField(reqBody.Database, reqBody.DatabaseRDBMS, "database", "database_rdbms")
	if err != nil {
		status, errType, title := middleware.MapServiceError(err)
		middleware.WriteJSONError(w, r, status, errType, title, err.Error())
		return
	}

	if reqBody.Compute == nil && reqBody.Storage == nil && reqBody.Network == nil && dbReq == nil && reqBody.DatabaseNoSQL == nil && reqBody.Kubernetes == nil && reqBody.Serverless == nil {
		middleware.WriteJSONError(w, r, http.StatusBadRequest, "https://cloudvitta.dev/errors/missing-categories", "Missing Categories", "At least one category ('compute', 'storage', 'network', 'database_rdbms', 'database_nosql', 'kubernetes', or 'serverless') must be specified.")
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

	if dbReq != nil {
		rawEngine := strings.TrimSpace(dbReq.Engine)
		if rawEngine != "" {
			canonical, err := databaseenginemap.ResolveCanonicalEngine(rawEngine)
			if err != nil || !databaseenginemap.IsSupportedStageEngine(canonical) {
				middleware.WriteJSONError(w, r, http.StatusBadRequest, "https://cloudvitta.dev/errors/invalid-parameter", "Invalid Parameters", "database_rdbms.engine must be a supported database engine (postgresql, mysql, sqlserver)")
				return
			}
			dbReq.Engine = canonical
		}
		if dbReq.VCPU <= 0 {
			middleware.WriteJSONError(w, r, http.StatusBadRequest, "https://cloudvitta.dev/errors/invalid-parameter", "Invalid Parameters", "database_rdbms.vcpu must be a positive number")
			return
		}
		if dbReq.RAMGB <= 0 {
			middleware.WriteJSONError(w, r, http.StatusBadRequest, "https://cloudvitta.dev/errors/invalid-parameter", "Invalid Parameters", "database_rdbms.ram_gb must be a positive number")
			return
		}
		if dbReq.StorageGB <= 0 {
			middleware.WriteJSONError(w, r, http.StatusBadRequest, "https://cloudvitta.dev/errors/invalid-parameter", "Invalid Parameters", "database_rdbms.storage_gb must be a positive number")
			return
		}
		if dbReq.StorageGB > 1000000 {
			middleware.WriteJSONError(w, r, http.StatusBadRequest, "https://cloudvitta.dev/errors/invalid-parameter", "Invalid Parameters", "database_rdbms.storage_gb exceeds maximum limit of 1,000,000 GB (1 PB)")
			return
		}
		if dbReq.IOPS != nil && *dbReq.IOPS <= 0 {
			middleware.WriteJSONError(w, r, http.StatusBadRequest, "https://cloudvitta.dev/errors/invalid-parameter", "Invalid Parameters", "database_rdbms.iops must be a positive integer")
			return
		}
	}

	if reqBody.DatabaseNoSQL != nil {
		rawModel := strings.TrimSpace(reqBody.DatabaseNoSQL.DataModel)
		if rawModel != "" {
			canonical, err := nosqldatamodelmap.ResolveCanonicalDataModel(rawModel)
			if err != nil || !nosqldatamodelmap.IsSupportedStageDataModel(canonical) {
				middleware.WriteJSONError(w, r, http.StatusBadRequest, "https://cloudvitta.dev/errors/invalid-parameter", "Invalid Parameters", "database_nosql.data_model must be a supported NoSQL data model (document, key_value, wide_column, graph, multi_model)")
				return
			}
			reqBody.DatabaseNoSQL.DataModel = canonical
		}

		rawPricingMode := strings.ToLower(strings.TrimSpace(reqBody.DatabaseNoSQL.PricingMode))
		if rawPricingMode == "" {
			rawPricingMode = "provisioned"
		}
		if rawPricingMode != "provisioned" && rawPricingMode != "on_demand" && rawPricingMode != "serverless" {
			middleware.WriteJSONError(w, r, http.StatusBadRequest, "https://cloudvitta.dev/errors/invalid-parameter", "Invalid Parameters", "database_nosql.pricing_mode must be provisioned, on_demand, or serverless")
			return
		}
		reqBody.DatabaseNoSQL.PricingMode = rawPricingMode

		if reqBody.DatabaseNoSQL.ReadUnits < 0 || reqBody.DatabaseNoSQL.ReadUnits > 10000000 {
			middleware.WriteJSONError(w, r, http.StatusBadRequest, "https://cloudvitta.dev/errors/invalid-parameter", "Invalid Parameters", "database_nosql.read_units must be a non-negative number up to 10000000")
			return
		}
		if reqBody.DatabaseNoSQL.WriteUnits < 0 || reqBody.DatabaseNoSQL.WriteUnits > 10000000 {
			middleware.WriteJSONError(w, r, http.StatusBadRequest, "https://cloudvitta.dev/errors/invalid-parameter", "Invalid Parameters", "database_nosql.write_units must be a non-negative number up to 10000000")
			return
		}
		if reqBody.DatabaseNoSQL.StorageGB < 0 || reqBody.DatabaseNoSQL.StorageGB > 1000000 {
			middleware.WriteJSONError(w, r, http.StatusBadRequest, "https://cloudvitta.dev/errors/invalid-parameter", "Invalid Parameters", "database_nosql.storage_gb must be a non-negative number up to 1000000")
			return
		}
	}

	if reqBody.Kubernetes != nil {
		rawTier := strings.TrimSpace(string(reqBody.Kubernetes.Tier))
		canonicalTier := kubernetestieremap.TierStandard
		if rawTier != "" {
			resolved, err := kubernetestieremap.ResolveCanonicalTier(rawTier)
			if err != nil || !kubernetestieremap.IsSupportedStageTier(resolved) {
				middleware.WriteJSONError(w, r, http.StatusBadRequest, "https://cloudvitta.dev/errors/invalid-parameter", "Invalid Parameters", "kubernetes.tier must be a supported Kubernetes tier (free, standard, extended_support)")
				return
			}
			canonicalTier = resolved
		}
		reqBody.Kubernetes.Tier = canonicalTier

		rawTopology := strings.ToLower(strings.TrimSpace(string(reqBody.Kubernetes.ClusterTopology)))
		if rawTopology != "" {
			switch domain.ClusterTopology(rawTopology) {
			case domain.ClusterTopologyZonal, domain.ClusterTopologyRegional, domain.ClusterTopologyAutopilot:
				reqBody.Kubernetes.ClusterTopology = domain.ClusterTopology(rawTopology)
			default:
				middleware.WriteJSONError(w, r, http.StatusBadRequest, "https://cloudvitta.dev/errors/invalid-parameter", "Invalid Parameters", "kubernetes.cluster_topology must be one of: zonal, regional, autopilot")
				return
			}
		}
	}

	var serverlessWorkload *service.ServerlessWorkload
	if reqBody.Serverless != nil {
		var rawReqs, rawMem, rawDur string
		if reqBody.Serverless.RequestsPerMonth != nil {
			rawReqs = reqBody.Serverless.RequestsPerMonth.String()
		}
		if reqBody.Serverless.MemoryMB != nil {
			rawMem = reqBody.Serverless.MemoryMB.String()
		}
		if reqBody.Serverless.ExecutionDurationMS != nil {
			rawDur = reqBody.Serverless.ExecutionDurationMS.String()
		}
		wl, err := service.ParseServerlessWorkload(reqBody.Serverless.Architecture, reqBody.Serverless.Tier, rawReqs, rawMem, rawDur)
		if err != nil {
			middleware.WriteJSONError(w, r, http.StatusBadRequest, "https://cloudvitta.dev/errors/invalid-parameter", "Invalid Parameters", err.Error())
			return
		}
		serverlessWorkload = &wl
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
		Region:        region,
		Currency:      currency,
		StrictFamily:  strictFamily,
		Compute:       reqBody.Compute,
		Storage:       reqBody.Storage,
		Network:       reqBody.Network,
		DatabaseRDBMS: dbReq,
		DatabaseNoSQL: reqBody.DatabaseNoSQL,
		Kubernetes:    reqBody.Kubernetes,
		Serverless:    serverlessWorkload,
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

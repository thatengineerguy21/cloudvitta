package rest

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/service"
	"github.com/thatengineerguy21/CloudVitta/internal/transport/rest/middleware"
)

// CatalogSummaryResponse defines the response schema for GET /api/v1/catalog/compute/summary.
type CatalogSummaryResponse struct {
	TotalInstances    int64                       `json:"total_instances"`
	ProviderTotals    map[string]int64            `json:"provider_totals"`
	CategoryBreakdown map[string]map[string]int64 `json:"category_breakdown"`
}

// CatalogInstancesResponse defines the response schema for GET /api/v1/catalog/compute/instances.
type CatalogInstancesResponse struct {
	Count     int                         `json:"count"`
	Total     int64                       `json:"total"`
	Instances []domain.ComputeCatalogItem `json:"instances"`
}

// CatalogSummaryHandler handles GET /api/v1/catalog/compute/summary.
type CatalogSummaryHandler struct {
	catalogSvc *service.CatalogService
}

// NewCatalogSummaryHandler creates a new CatalogSummaryHandler.
func NewCatalogSummaryHandler(catalogSvc *service.CatalogService) http.Handler {
	return &CatalogSummaryHandler{catalogSvc: catalogSvc}
}

// @Summary      Get compute instance catalog summary
// @Description  Returns total instances, provider counts, and category breakdowns across all cloud providers.
// @Tags         catalog
// @Produce      json
// @Success      200  {object}  CatalogSummaryResponse
// @Failure      500  {object}  middleware.RFC7807Error
// @Router       /api/v1/catalog/compute/summary [get]
func (h *CatalogSummaryHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodGet {
		middleware.WriteJSONError(w, r, http.StatusMethodNotAllowed, "https://cloudvitta.dev/errors/method-not-allowed", "Method Not Allowed", "Only GET method is supported")
		return
	}

	summary, err := h.catalogSvc.GetCatalogSummary(r.Context())
	if err != nil {
		middleware.WriteJSONError(w, r, http.StatusInternalServerError, "https://cloudvitta.dev/errors/internal", "Internal Server Error", err.Error())
		return
	}

	resp := CatalogSummaryResponse{
		TotalInstances:    summary.TotalInstances,
		ProviderTotals:    summary.ProviderTotals,
		CategoryBreakdown: summary.CategoryBreakdown,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// CatalogInstancesHandler handles GET /api/v1/catalog/compute/instances.
type CatalogInstancesHandler struct {
	catalogSvc *service.CatalogService
}

// NewCatalogInstancesHandler creates a new CatalogInstancesHandler.
func NewCatalogInstancesHandler(catalogSvc *service.CatalogService) http.Handler {
	return &CatalogInstancesHandler{catalogSvc: catalogSvc}
}

// @Summary      Search and filter compute instance catalog
// @Description  Returns paginated virtual machine specifications filtered by provider, category, family, and vCPU/RAM ranges.
// @Tags         catalog
// @Produce      json
// @Param        provider         query     string  false  "Cloud provider (aws, azure, gcp, etc.)"
// @Param        category         query     string  false  "Instance category (general_purpose, compute_optimized, etc.)"
// @Param        instance_family  query     string  false  "Instance family (t3, c5, Standard_D, etc.)"
// @Param        min_vcpu         query     number  false  "Minimum vCPU count"
// @Param        max_vcpu         query     number  false  "Maximum vCPU count"
// @Param        min_memory_gib   query     number  false  "Minimum RAM in GiB"
// @Param        max_memory_gib   query     number  false  "Maximum RAM in GiB"
// @Param        limit            query     int     false  "Limit (default: 50, max: 200)"
// @Param        offset           query     int     false  "Offset (default: 0)"
// @Success      200              {object}  CatalogInstancesResponse
// @Failure      400              {object}  middleware.RFC7807Error
// @Failure      500              {object}  middleware.RFC7807Error
// @Router       /api/v1/catalog/compute/instances [get]
func (h *CatalogInstancesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodGet {
		middleware.WriteJSONError(w, r, http.StatusMethodNotAllowed, "https://cloudvitta.dev/errors/method-not-allowed", "Method Not Allowed", "Only GET method is supported")
		return
	}

	q := r.URL.Query()
	filter := domain.CatalogFilter{
		Limit:  50,
		Offset: 0,
	}

	if p := q.Get("provider"); p != "" {
		filter.Provider = &p
	}
	if c := q.Get("category"); c != "" {
		filter.Category = &c
	}
	if f := q.Get("instance_family"); f != "" {
		filter.InstanceFamily = &f
	}

	if s := q.Get("min_vcpu"); s != "" {
		v, err := strconv.ParseFloat(s, 64)
		if err != nil || v < 0 {
			middleware.WriteJSONError(w, r, http.StatusBadRequest, "https://cloudvitta.dev/errors/invalid-parameter", "Invalid Parameter", "min_vcpu must be a non-negative number")
			return
		}
		filter.MinVCPU = &v
	}
	if s := q.Get("max_vcpu"); s != "" {
		v, err := strconv.ParseFloat(s, 64)
		if err != nil || v < 0 {
			middleware.WriteJSONError(w, r, http.StatusBadRequest, "https://cloudvitta.dev/errors/invalid-parameter", "Invalid Parameter", "max_vcpu must be a non-negative number")
			return
		}
		filter.MaxVCPU = &v
	}
	if s := q.Get("min_memory_gib"); s != "" {
		v, err := strconv.ParseFloat(s, 64)
		if err != nil || v < 0 {
			middleware.WriteJSONError(w, r, http.StatusBadRequest, "https://cloudvitta.dev/errors/invalid-parameter", "Invalid Parameter", "min_memory_gib must be a non-negative number")
			return
		}
		filter.MinMemoryGiB = &v
	}
	if s := q.Get("max_memory_gib"); s != "" {
		v, err := strconv.ParseFloat(s, 64)
		if err != nil || v < 0 {
			middleware.WriteJSONError(w, r, http.StatusBadRequest, "https://cloudvitta.dev/errors/invalid-parameter", "Invalid Parameter", "max_memory_gib must be a non-negative number")
			return
		}
		filter.MaxMemoryGiB = &v
	}

	if s := q.Get("limit"); s != "" {
		lim, err := strconv.Atoi(s)
		if err != nil || lim <= 0 {
			middleware.WriteJSONError(w, r, http.StatusBadRequest, "https://cloudvitta.dev/errors/invalid-parameter", "Invalid Parameter", "limit must be a positive integer")
			return
		}
		filter.Limit = int32(lim)
	}
	if s := q.Get("offset"); s != "" {
		off, err := strconv.Atoi(s)
		if err != nil || off < 0 {
			middleware.WriteJSONError(w, r, http.StatusBadRequest, "https://cloudvitta.dev/errors/invalid-parameter", "Invalid Parameter", "offset must be a non-negative integer")
			return
		}
		filter.Offset = int32(off)
	}

	instances, total, err := h.catalogSvc.ListCatalogInstances(r.Context(), filter)
	if err != nil {
		middleware.WriteJSONError(w, r, http.StatusInternalServerError, "https://cloudvitta.dev/errors/internal", "Internal Server Error", err.Error())
		return
	}

	resp := CatalogInstancesResponse{
		Count:     len(instances),
		Total:     total,
		Instances: instances,
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

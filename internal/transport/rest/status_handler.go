package rest

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"time"

	"github.com/thatengineerguy21/CloudVitta/internal/service"
	"github.com/thatengineerguy21/CloudVitta/internal/transport/rest/middleware"
)

// DLQStatusResponse represents the DLQ failure state of an ingestion job for a provider and category.
type DLQStatusResponse struct {
	Status              string    `json:"status" example:"failed"`
	LastError           string    `json:"last_error" example:"503 Service Unavailable"`
	ConsecutiveFailures int       `json:"consecutive_failures" example:"3"`
	Timestamp           time.Time `json:"timestamp"`
}

// CategoryStatusResponse represents data freshness, counts, and DLQ status for a specific service category.
type CategoryStatusResponse struct {
	Category                string             `json:"category" example:"compute"`
	Supported               bool               `json:"supported" example:"true"`
	LastFetchedAt           *time.Time         `json:"last_fetched_at,omitempty"`
	LastSeenAt              *time.Time         `json:"last_seen_at,omitempty"`
	ObservationCount        int64              `json:"observation_count" example:"450"`
	Stale                   bool               `json:"stale" example:"false"`
	StalenessThresholdHours float64            `json:"staleness_threshold_hours" example:"168"`
	DLQ                     *DLQStatusResponse `json:"dlq,omitempty"`
}

// ProviderStatusWarningResponse contains information regarding unsupported or uningested provider states.
type ProviderStatusWarningResponse struct {
	Provider string `json:"provider" example:"oracle"`
	Code     string `json:"code" example:"not_yet_ingested"`
	Message  string `json:"message" example:"Oracle OCI ingestion lands in stage 4."`
}

// ProviderStatusResponse represents the aggregate operational and data freshness status of a cloud provider.
type ProviderStatusResponse struct {
	Provider            string                            `json:"provider" example:"aws"`
	Status              string                            `json:"status" example:"healthy"`
	LastSuccessfulFetch *time.Time                        `json:"last_successful_fetch,omitempty"`
	Stale               bool                              `json:"stale" example:"false"`
	Categories          map[string]CategoryStatusResponse `json:"categories"`
	Warnings            []ProviderStatusWarningResponse   `json:"warnings"`
}

// StatusHandler handles provider status and data freshness inspection requests.
type StatusHandler struct {
	freshnessSvc *service.FreshnessService
}

// NewStatusHandler constructs a new StatusHandler.
func NewStatusHandler(freshnessSvc *service.FreshnessService) *StatusHandler {
	return &StatusHandler{
		freshnessSvc: freshnessSvc,
	}
}

// ServeHTTP handles GET /api/v1/providers/{provider}/status
// @Summary Get cloud provider operational status and data freshness
// @Description Returns the latest successful fetch timestamp, per-category observation count, staleness flag, and active DLQ failure records for the specified cloud provider.
// @Tags providers
// @Produce json
// @Param provider path string true "Cloud provider identifier (e.g. aws, azure, gcp, oracle, ibm, alibaba, digitalocean)"
// @Success 200 {object} ProviderStatusResponse "Provider status and category freshness details"
// @Failure 400 {object} middleware.RFC7807Error "Invalid Parameters"
// @Failure 404 {object} middleware.RFC7807Error "Provider Not Found"
// @Failure 500 {object} middleware.RFC7807Error "Internal Server Error"
// @Router /api/v1/providers/{provider}/status [get]
func (h *StatusHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		middleware.WriteJSONError(w, r, http.StatusMethodNotAllowed, "https://cloudvitta.dev/errors/method-not-allowed", "Method Not Allowed", "Only GET method is supported")
		return
	}

	provider := strings.TrimSpace(r.PathValue("provider"))
	if provider == "" {
		middleware.WriteJSONError(w, r, http.StatusBadRequest, "https://cloudvitta.dev/errors/invalid-parameter", "Invalid Parameters", "provider path parameter is required")
		return
	}

	if h.freshnessSvc == nil {
		middleware.WriteJSONError(w, r, http.StatusInternalServerError, "https://cloudvitta.dev/errors/internal-server-error", "Internal Server Error", "freshness service unavailable")
		return
	}

	status, err := h.freshnessSvc.GetProviderStatus(r.Context(), provider)
	if err != nil {
		if errors.Is(err, service.ErrProviderNotFound) {
			middleware.WriteJSONError(w, r, http.StatusNotFound, "https://cloudvitta.dev/errors/provider-not-found", "Provider Not Found", "provider '"+provider+"' is not recognized")
			return
		}
		middleware.WriteJSONError(w, r, http.StatusInternalServerError, "https://cloudvitta.dev/errors/internal-server-error", "Internal Server Error", "An internal server error occurred while retrieving provider status")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(status)
}

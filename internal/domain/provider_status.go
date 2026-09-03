package domain

import "time"

// DLQStatus represents the DLQ failure state of an ingestion job for a provider and category.
type DLQStatus struct {
	Status              string    `json:"status"` // "failed" or "blocked"
	LastError           string    `json:"last_error"`
	ConsecutiveFailures int       `json:"consecutive_failures"`
	Timestamp           time.Time `json:"timestamp"`
}

// CategoryStatus represents data freshness, counts, and DLQ status for a specific service category.
type CategoryStatus struct {
	Category                string     `json:"category"`
	Supported               bool       `json:"supported"`
	LastFetchedAt           *time.Time `json:"last_fetched_at,omitempty"`
	LastSeenAt              *time.Time `json:"last_seen_at,omitempty"`
	ObservationCount        int64      `json:"observation_count"`
	Stale                   bool       `json:"stale"`
	StalenessThresholdHours float64    `json:"staleness_threshold_hours"`
	DLQ                     *DLQStatus `json:"dlq,omitempty"`
}

// ProviderStatus represents the aggregate operational and data freshness status of a cloud provider.
type ProviderStatus struct {
	Provider            string                    `json:"provider"`
	Status              string                    `json:"status"` // "healthy", "partially_healthy", "degraded", "stale", "blocked", "not_yet_ingested"
	LastSuccessfulFetch *time.Time                `json:"last_successful_fetch,omitempty"`
	Stale               bool                      `json:"stale"`
	Categories          map[string]CategoryStatus `json:"categories"`
	Warnings            []ProviderStatusWarning   `json:"warnings,omitempty"`
}

// ProviderStatusWarning contains information regarding unsupported or uningested provider states.
type ProviderStatusWarning struct {
	Provider string `json:"provider"`
	Code     string `json:"code"`
	Message  string `json:"message"`
}

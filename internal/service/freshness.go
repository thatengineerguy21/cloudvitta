package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/thatengineerguy21/CloudVitta/internal/dlq"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/store"
)

// DLQReader defines the interface for reading DLQ failure records.
type DLQReader interface {
	Get(ctx context.Context, provider, category string) (dlq.Entry, error)
}

// FreshnessOption allows configuring optional dependencies and thresholds for FreshnessService.
type FreshnessOption func(*FreshnessService)

// WithNowFunc injects a custom clock function for deterministic testing.
func WithNowFunc(nowFunc func() time.Time) FreshnessOption {
	return func(s *FreshnessService) {
		s.nowFunc = nowFunc
	}
}

// WithDefaultThreshold sets the fallback staleness threshold duration.
func WithDefaultThreshold(d time.Duration) FreshnessOption {
	return func(s *FreshnessService) {
		if d > 0 {
			s.defaultThreshold = d
		}
	}
}

// WithCustomThreshold sets a per-provider or per-provider:category threshold override.
func WithCustomThreshold(key string, d time.Duration) FreshnessOption {
	return func(s *FreshnessService) {
		if s.customThresholds == nil {
			s.customThresholds = make(map[string]time.Duration)
		}
		s.customThresholds[key] = d
	}
}

// FreshnessService evaluates pricing data freshness and aggregates provider status.
type FreshnessService struct {
	queries          store.Querier
	dlq              DLQReader
	defaultThreshold time.Duration
	customThresholds map[string]time.Duration
	nowFunc          func() time.Time
}

// NewFreshnessService constructs a new FreshnessService.
func NewFreshnessService(queries store.Querier, dlqReader DLQReader, opts ...FreshnessOption) *FreshnessService {
	s := &FreshnessService{
		queries:          queries,
		dlq:              dlqReader,
		defaultThreshold: 168 * time.Hour, // 7 days (weekly cadence per PRD §17)
		customThresholds: make(map[string]time.Duration),
		nowFunc:          time.Now,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Threshold returns the applicable staleness threshold for the given provider and category.
func (s *FreshnessService) Threshold(provider, category string) time.Duration {
	if s == nil {
		return 168 * time.Hour
	}
	if s.customThresholds != nil {
		if d, ok := s.customThresholds[fmt.Sprintf("%s:%s", provider, category)]; ok && d > 0 {
			return d
		}
		if d, ok := s.customThresholds[provider]; ok && d > 0 {
			return d
		}
	}
	if s.defaultThreshold > 0 {
		return s.defaultThreshold
	}
	return 168 * time.Hour
}

// IsStale evaluates whether observation fetchedAt is older than the configured threshold.
func (s *FreshnessService) IsStale(provider, category string, fetchedAt time.Time) bool {
	if fetchedAt.IsZero() {
		return true
	}
	now := time.Now
	if s != nil && s.nowFunc != nil {
		now = s.nowFunc
	}
	threshold := s.Threshold(provider, category)
	return now().Sub(fetchedAt) > threshold
}

func stage3WarningMessage(provider string) (string, bool) {
	switch provider {
	case "oracle":
		return "Oracle OCI ingestion lands in stage 3.", true
	case "ibm":
		return "IBM Cloud ingestion lands in stage 3.", true
	case "alibaba":
		return "Alibaba Cloud ingestion lands in stage 3.", true
	case "digitalocean":
		return "DigitalOcean ingestion lands in stage 3.", true
	default:
		return "", false
	}
}

// GetProviderStatus builds the complete operational and freshness status for a provider.
func (s *FreshnessService) GetProviderStatus(ctx context.Context, provider string) (*domain.ProviderStatus, error) {
	p := strings.ToLower(strings.TrimSpace(provider))
	if msg, ok := stage3WarningMessage(p); ok {
		return &domain.ProviderStatus{
			Provider:   p,
			Status:     "not_yet_ingested",
			Stale:      true,
			Categories: make(map[string]domain.CategoryStatus),
			Warnings: []domain.ProviderStatusWarning{
				{
					Provider: p,
					Code:     "not_yet_ingested",
					Message:  msg,
				},
			},
		}, nil
	}

	if p != "aws" && p != "azure" && p != "gcp" {
		return nil, ErrProviderNotFound
	}

	supportedCategories := SupportedCategoriesForProvider(p)
	catMap := make(map[string]domain.CategoryStatus)

	dbCatMap := make(map[string]store.GetProviderCategoryStatusRow)
	if s != nil && s.queries != nil {
		rows, err := s.queries.GetProviderCategoryStatus(ctx, p)
		if err != nil {
			return nil, fmt.Errorf("freshness service: get category status: %w", err)
		}
		for _, row := range rows {
			dbCatMap[row.ServiceCategory] = row
		}
	}

	var maxFetchedAt *time.Time
	var hasBlockedDLQ bool
	var anyCategoryStale bool
	var allCategoriesStaleOrEmpty = true
	var allCategoriesHealthy = true

	for _, cat := range supportedCategories {
		threshold := s.Threshold(p, cat)
		thresholdHours := threshold.Hours()

		var lastFetchedAt *time.Time
		var lastSeenAt *time.Time
		var count int64
		isStale := true

		if row, exists := dbCatMap[cat]; exists {
			if row.LastFetchedAt.Valid && !row.LastFetchedAt.Time.IsZero() {
				t := row.LastFetchedAt.Time
				lastFetchedAt = &t
				count = row.ObservationCount
				isStale = s.IsStale(p, cat, t)
				if maxFetchedAt == nil || t.After(*maxFetchedAt) {
					tCopy := t
					maxFetchedAt = &tCopy
				}
			}
			if row.LastSeenAt.Valid && !row.LastSeenAt.Time.IsZero() {
				st := row.LastSeenAt.Time
				lastSeenAt = &st
			}
		}

		var dlqStatus *domain.DLQStatus
		if s != nil && s.dlq != nil {
			entry, err := s.dlq.Get(ctx, p, cat)
			if err == nil && entry.LastError != "" {
				dlqStatus = &domain.DLQStatus{
					Status:              entry.Status,
					LastError:           entry.LastError,
					ConsecutiveFailures: entry.ConsecutiveFailures,
					Timestamp:           entry.Timestamp,
				}
				if entry.Status == "blocked" {
					hasBlockedDLQ = true
				}
			}
		}

		catStatus := domain.CategoryStatus{
			Category:                cat,
			Supported:               true,
			LastFetchedAt:           lastFetchedAt,
			LastSeenAt:              lastSeenAt,
			ObservationCount:        count,
			Stale:                   isStale,
			StalenessThresholdHours: thresholdHours,
			DLQ:                     dlqStatus,
		}
		catMap[cat] = catStatus

		if isStale {
			anyCategoryStale = true
		}
		if !isStale && count > 0 && dlqStatus == nil {
			allCategoriesStaleOrEmpty = false
		}
		if isStale || count == 0 || dlqStatus != nil {
			allCategoriesHealthy = false
		}
	}

	var status string
	if hasBlockedDLQ {
		status = "blocked"
	} else if allCategoriesHealthy && len(supportedCategories) > 0 {
		status = "healthy"
	} else if allCategoriesStaleOrEmpty {
		status = "stale"
	} else {
		status = "degraded"
	}

	return &domain.ProviderStatus{
		Provider:            p,
		Status:              status,
		LastSuccessfulFetch: maxFetchedAt,
		Stale:               anyCategoryStale || len(supportedCategories) == 0,
		Categories:          catMap,
		Warnings:            []domain.ProviderStatusWarning{},
	}, nil
}

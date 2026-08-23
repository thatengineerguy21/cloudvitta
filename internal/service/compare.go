package service

import (
	"context"
	"errors"
	"time"

	"github.com/shopspring/decimal"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
)

// CategoryComparisonItem holds a single provider's match and calculation outcome for a single category comparison.
type CategoryComparisonItem struct {
	Provider          string
	SkuID             string
	MatchedCompute    domain.ComputeAttributes
	MatchedStorage    domain.StorageAttributes
	MatchedNetwork    domain.NetworkAttributes
	MatchedDatabase   domain.DatabaseRDBMSAttributes
	MatchedNoSQL      domain.DatabaseNoSQLAttributes
	MatchedKubernetes domain.KubernetesAttributes
	MatchQuality      string
	MatchDeltaPct     float64
	MissingAttributes []string
	PriceAmount       decimal.Decimal
	PriceCurrency     string
	Unit              string
	HourlyCost        decimal.Decimal
	MonthlyCost       decimal.Decimal
	FetchedAt         time.Time
	Stale             bool
}

// ComparisonResult holds the aggregated comparison items and warnings across providers.
type ComparisonResult struct {
	Results  []CategoryComparisonItem
	Warnings []CalculateWarning
}

// Compare orchestrates fetching, matching, and cost calculation across all supported providers
// for a single category (compute, storage, or network), returning unified results and warnings.
func (s *PricingService) Compare(ctx context.Context, category, region string, target MatchTarget) (*ComparisonResult, error) {
	if region == "" {
		region = "us-east"
	}

	providers := SupportedProviders()
	var results []CategoryComparisonItem
	var warnings []CalculateWarning
	var providerErrors int

	for _, prov := range providers {
		catResult, err := s.MatchAndCalculate(ctx, prov, category, region, target)
		if err != nil {
			switch {
			case errors.Is(err, ErrCategoryNotSupported):
				warnings = append(warnings, CalculateWarning{
					Provider: prov,
					Code:     "category_not_supported",
					Message:  category + " category is not supported by " + prov,
				})
			case errors.Is(err, ErrEngineMismatch):
				warnings = append(warnings, CalculateWarning{
					Provider: prov,
					Code:     "engine_mismatch_excluded",
					Message:  "Database candidate was excluded due to engine mismatch.",
				})
			case errors.Is(err, ErrNoMatchFound):
				warnings = append(warnings, CalculateWarning{
					Provider: prov,
					Code:     "no_match",
					Message:  "No " + category + " SKU matched the requested spec within acceptable thresholds.",
				})
			default:
				providerErrors++
				warnings = append(warnings, CalculateWarning{
					Provider: prov,
					Code:     "fetch_failed",
					Message:  err.Error(),
				})
			}
			continue
		}

		if catResult == nil {
			warnings = append(warnings, CalculateWarning{
				Provider: prov,
				Code:     "no_data_available",
				Message:  "No " + category + " pricing data available for this region.",
			})
			continue
		}

		obs := catResult.MatchResult.Observation
		results = append(results, CategoryComparisonItem{
			Provider:          obs.Provider,
			SkuID:             obs.SkuID,
			MatchedCompute:    obs.Attributes,
			MatchedStorage:    obs.StorageAttributes,
			MatchedNetwork:    obs.NetworkAttributes,
			MatchedDatabase:   obs.DatabaseRDBMSAttributes,
			MatchedNoSQL:      obs.DatabaseNoSQLAttributes,
			MatchedKubernetes: obs.KubernetesAttributes,
			MatchQuality:      catResult.MatchResult.MatchQuality,
			MatchDeltaPct:     catResult.MatchResult.MatchDeltaPct,
			MissingAttributes: catResult.MatchResult.MissingAttributes,
			PriceAmount:       obs.PriceAmount,
			PriceCurrency:     obs.PriceCurrency,
			Unit:              catResult.Unit,
			HourlyCost:        catResult.HourlyCost,
			MonthlyCost:       catResult.MonthlyCost,
			FetchedAt:         obs.FetchedAt,
			Stale:             catResult.Stale,
		})
	}

	if len(results) == 0 && providerErrors == len(providers) {
		return nil, ErrProviderUnavailable
	}

	return &ComparisonResult{
		Results:  results,
		Warnings: warnings,
	}, nil
}

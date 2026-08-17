package service

import (
	"context"

	"github.com/shopspring/decimal"
)

// CalculateStorageMonthlyCost calculates the estimated monthly storage cost given unit price and size in GB.
// This encapsulates pricing arithmetic in the service layer (08-CONSISTENCY-RULES.md).
func CalculateStorageMonthlyCost(unitPrice, sizeGB decimal.Decimal) decimal.Decimal {
	return unitPrice.Mul(sizeGB)
}

// CalculateNetworkMonthlyCost calculates the estimated monthly network egress cost given unit price and egress in GB.
// This encapsulates pricing arithmetic in the service layer (08-CONSISTENCY-RULES.md).
func CalculateNetworkMonthlyCost(unitPrice, egressGB decimal.Decimal) decimal.Decimal {
	return unitPrice.Mul(egressGB)
}

// HoursInMonth defines the standard average hours in a month (365 days * 24 hours / 12 months = 730 hours).
var HoursInMonth = decimal.NewFromInt(730)

// CalculateStorageHourlyCost calculates normalized hourly storage cost from unit price ($/GB-mo) and size in GB.
// Formula: (unitPrice * sizeGB) / 730
func CalculateStorageHourlyCost(unitPrice, sizeGB decimal.Decimal) decimal.Decimal {
	return CalculateStorageMonthlyCost(unitPrice, sizeGB).Div(HoursInMonth)
}

// CalculateNetworkHourlyCost calculates normalized hourly network egress cost from unit price ($/GB) and egress in GB.
// Formula: (unitPrice * egressGB) / 730
func CalculateNetworkHourlyCost(unitPrice, egressGB decimal.Decimal) decimal.Decimal {
	return CalculateNetworkMonthlyCost(unitPrice, egressGB).Div(HoursInMonth)
}

// CategoryPricingResult contains the match result along with computed costs.
type CategoryPricingResult struct {
	MatchResult *MatchResult
	HourlyCost  decimal.Decimal
	MonthlyCost decimal.Decimal
	Unit        string
	Stale       bool
}

// MatchAndCalculate encapsulates the fetch, matching, and cost calculation for any category.
// It acts as the single source of truth for tying MatchObservations to Calculate*Costs.
func (s *PricingService) MatchAndCalculate(ctx context.Context, provider, category, region string, target MatchTarget) (*CategoryPricingResult, error) {
	if !IsProviderCategorySupported(provider, category) {
		return nil, ErrCategoryNotSupported
	}

	obsList, err := s.GetPrices(ctx, provider, category, region)
	if err != nil {
		return nil, err
	}
	if len(obsList) == 0 {
		return nil, nil // No data available
	}

	var scorer CategoryScorer
	switch category {
	case "compute":
		scorer = ComputeScorer{}
	case "storage":
		scorer = StorageScorer{}
	case "network":
		scorer = NetworkScorer{}
	default:
		return nil, ErrInvalidParameters
	}

	matchResult := MatchObservations(scorer, obsList, target, ThresholdsForCategory(category))
	if matchResult == nil {
		return nil, ErrNoMatchFound
	}

	var hourlyCost, monthlyCost decimal.Decimal
	var unit string

	switch category {
	case "compute":
		hourlyCost = matchResult.Observation.PriceAmount
		monthlyCost = hourlyCost.Mul(HoursInMonth)
		unit = matchResult.Observation.Unit
		if unit == "" {
			unit = "hour"
		}
	case "storage":
		sizeGB := decimal.NewFromFloat(target.SizeGB)
		if sizeGB.LessThanOrEqual(decimal.Zero) {
			sizeGB = decimal.NewFromInt(1)
		}
		hourlyCost = CalculateStorageHourlyCost(matchResult.Observation.PriceAmount, sizeGB)
		monthlyCost = CalculateStorageMonthlyCost(matchResult.Observation.PriceAmount, sizeGB)
		unit = matchResult.Observation.Unit
		if unit == "" {
			unit = "GB-Mo"
		}
	case "network":
		egressGB := decimal.NewFromFloat(target.EgressGB)
		if egressGB.LessThanOrEqual(decimal.Zero) {
			egressGB = decimal.NewFromInt(1)
		}
		hourlyCost = CalculateNetworkHourlyCost(matchResult.Observation.PriceAmount, egressGB)
		monthlyCost = CalculateNetworkMonthlyCost(matchResult.Observation.PriceAmount, egressGB)
		unit = matchResult.Observation.Unit
		if unit == "" {
			unit = "GB"
		}
	}

	var isStale bool
	if s.freshnessSvc != nil {
		isStale = s.freshnessSvc.IsStale(provider, category, matchResult.Observation.FetchedAt)
	}

	return &CategoryPricingResult{
		MatchResult: matchResult,
		HourlyCost:  hourlyCost,
		MonthlyCost: monthlyCost,
		Unit:        unit,
		Stale:       isStale,
	}, nil
}

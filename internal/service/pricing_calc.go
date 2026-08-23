package service

import (
	"context"

	"github.com/shopspring/decimal"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
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

// CalculateIOPSMonthlyCost calculates estimated monthly provisioned IOPS cost from unit price ($/IOPS-mo) and IOPS count.
// This encapsulates pricing arithmetic in the service layer (08-CONSISTENCY-RULES.md).
func CalculateIOPSMonthlyCost(unitPrice decimal.Decimal, iops int64) decimal.Decimal {
	return unitPrice.Mul(decimal.NewFromInt(iops))
}

// CalculateIOPSHourlyCost calculates normalized hourly provisioned IOPS cost from unit price ($/IOPS-mo) and IOPS count.
// Formula: (unitPrice * iops) / 730
func CalculateIOPSHourlyCost(unitPrice decimal.Decimal, iops int64) decimal.Decimal {
	return CalculateIOPSMonthlyCost(unitPrice, iops).Div(HoursInMonth)
}

// CategoryPricingResult contains the match result along with computed costs and warnings.
type CategoryPricingResult struct {
	MatchResult *MatchResult
	HourlyCost  decimal.Decimal
	MonthlyCost decimal.Decimal
	Unit        string
	Stale       bool
	Warnings    []CalculateWarning
}

// CategoryPricingHandler encapsulates category-specific matching and cost arithmetic.
type CategoryPricingHandler struct {
	Match          func(obsList []domain.PriceObservation, target MatchTarget, thresholds CategoryThresholds) (*MatchResult, error)
	CalculateCosts func(match *MatchResult, target MatchTarget) (hourlyCost, monthlyCost decimal.Decimal, unit string, warnings []CalculateWarning)
}

var categoryPricingHandlers = map[string]CategoryPricingHandler{}

// RegisterCategoryPricingHandler registers a matching and cost calculation handler for a service category.
func RegisterCategoryPricingHandler(category string, handler CategoryPricingHandler) {
	categoryPricingHandlers[category] = handler
}

func init() {
	RegisterCategoryPricingHandler("compute", CategoryPricingHandler{
		Match: func(obsList []domain.PriceObservation, target MatchTarget, thresholds CategoryThresholds) (*MatchResult, error) {
			res := MatchObservations(ComputeScorer{}, obsList, target, thresholds)
			if res == nil {
				return nil, ErrNoMatchFound
			}
			return res, nil
		},
		CalculateCosts: func(match *MatchResult, target MatchTarget) (decimal.Decimal, decimal.Decimal, string, []CalculateWarning) {
			hourlyCost := match.Observation.PriceAmount
			monthlyCost := hourlyCost.Mul(HoursInMonth)
			unit := match.Observation.Unit
			if unit == "" {
				unit = "hour"
			}
			return hourlyCost, monthlyCost, unit, nil
		},
	})

	RegisterCategoryPricingHandler("storage", CategoryPricingHandler{
		Match: func(obsList []domain.PriceObservation, target MatchTarget, thresholds CategoryThresholds) (*MatchResult, error) {
			res := MatchObservations(StorageScorer{}, obsList, target, thresholds)
			if res == nil {
				return nil, ErrNoMatchFound
			}
			return res, nil
		},
		CalculateCosts: func(match *MatchResult, target MatchTarget) (decimal.Decimal, decimal.Decimal, string, []CalculateWarning) {
			sizeGB := decimal.NewFromFloat(target.SizeGB)
			if sizeGB.LessThanOrEqual(decimal.Zero) {
				sizeGB = decimal.NewFromInt(1)
			}
			hourlyCost := CalculateStorageHourlyCost(match.Observation.PriceAmount, sizeGB)
			monthlyCost := CalculateStorageMonthlyCost(match.Observation.PriceAmount, sizeGB)
			unit := match.Observation.Unit
			if unit == "" {
				unit = "GB-Mo"
			}
			return hourlyCost, monthlyCost, unit, nil
		},
	})

	RegisterCategoryPricingHandler("network", CategoryPricingHandler{
		Match: func(obsList []domain.PriceObservation, target MatchTarget, thresholds CategoryThresholds) (*MatchResult, error) {
			res := MatchObservations(NetworkScorer{}, obsList, target, thresholds)
			if res == nil {
				return nil, ErrNoMatchFound
			}
			return res, nil
		},
		CalculateCosts: func(match *MatchResult, target MatchTarget) (decimal.Decimal, decimal.Decimal, string, []CalculateWarning) {
			egressGB := decimal.NewFromFloat(target.EgressGB)
			if egressGB.LessThanOrEqual(decimal.Zero) {
				egressGB = decimal.NewFromInt(1)
			}
			hourlyCost := CalculateNetworkHourlyCost(match.Observation.PriceAmount, egressGB)
			monthlyCost := CalculateNetworkMonthlyCost(match.Observation.PriceAmount, egressGB)
			unit := match.Observation.Unit
			if unit == "" {
				unit = "GB"
			}
			return hourlyCost, monthlyCost, unit, nil
		},
	})

	RegisterCategoryPricingHandler("database_rdbms", CategoryPricingHandler{
		Match: func(obsList []domain.PriceObservation, target MatchTarget, thresholds CategoryThresholds) (*MatchResult, error) {
			return MatchDatabaseObservations(obsList, target, thresholds)
		},
		CalculateCosts: func(match *MatchResult, target MatchTarget) (decimal.Decimal, decimal.Decimal, string, []CalculateWarning) {
			hourlyCost := match.Observation.PriceAmount
			monthlyCost := hourlyCost.Mul(HoursInMonth)
			unit := match.Observation.Unit
			if unit == "" {
				unit = "hour"
			}
			return hourlyCost, monthlyCost, unit, nil
		},
	})

	RegisterCategoryPricingHandler("database_nosql", CategoryPricingHandler{
		Match: func(obsList []domain.PriceObservation, target MatchTarget, thresholds CategoryThresholds) (*MatchResult, error) {
			return MatchNoSQLObservations(obsList, target, thresholds)
		},
		CalculateCosts: func(match *MatchResult, target MatchTarget) (decimal.Decimal, decimal.Decimal, string, []CalculateWarning) {
			hourlyCost := match.Observation.PriceAmount
			monthlyCost := hourlyCost.Mul(HoursInMonth)
			unit := match.Observation.Unit
			if unit == "" {
				unit = "hour"
			}
			return hourlyCost, monthlyCost, unit, nil
		},
	})

	RegisterCategoryPricingHandler("kubernetes", CategoryPricingHandler{
		Match: func(obsList []domain.PriceObservation, target MatchTarget, thresholds CategoryThresholds) (*MatchResult, error) {
			return MatchKubernetesObservations(obsList, target, thresholds)
		},
		CalculateCosts: func(match *MatchResult, target MatchTarget) (decimal.Decimal, decimal.Decimal, string, []CalculateWarning) {
			hourlyCost, warnings := AdjustKubernetesCost(match.Observation.Provider, match.Observation.PriceAmount, target)
			monthlyCost := hourlyCost.Mul(HoursInMonth)
			unit := match.Observation.Unit
			if unit == "" {
				unit = "hour"
			}
			return hourlyCost, monthlyCost, unit, warnings
		},
	})

	RegisterCategoryPricingHandler("serverless", CategoryPricingHandler{
		Match: func(obsList []domain.PriceObservation, target MatchTarget, thresholds CategoryThresholds) (*MatchResult, error) {
			return MatchServerlessObservations(obsList, target, thresholds)
		},
		CalculateCosts: func(match *MatchResult, target MatchTarget) (decimal.Decimal, decimal.Decimal, string, []CalculateWarning) {
			hourlyCost := match.Observation.PriceAmount
			monthlyCost := hourlyCost.Mul(HoursInMonth)
			unit := match.Observation.Unit
			if unit == "" {
				unit = "hour"
			}
			return hourlyCost, monthlyCost, unit, nil
		},
	})
}

// MatchAndCalculate encapsulates the fetch, matching, and cost calculation for any category.
// It acts as the single source of truth for tying MatchObservations to Calculate*Costs.
func (s *PricingService) MatchAndCalculate(ctx context.Context, provider, category, region string, target MatchTarget) (*CategoryPricingResult, error) {
	if !IsProviderCategorySupported(provider, category) {
		return nil, ErrCategoryNotSupported
	}

	handler, ok := categoryPricingHandlers[category]
	if !ok {
		return nil, ErrInvalidParameters
	}

	obsList, err := s.GetPrices(ctx, provider, category, region)
	if err != nil {
		return nil, err
	}
	if len(obsList) == 0 {
		return nil, nil // No data available
	}

	matchResult, err := handler.Match(obsList, target, ThresholdsForCategory(category))
	if err != nil {
		return nil, err
	}
	if matchResult == nil {
		return nil, ErrNoMatchFound
	}

	hourlyCost, monthlyCost, unit, warnings := handler.CalculateCosts(matchResult, target)

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
		Warnings:    warnings,
	}, nil
}

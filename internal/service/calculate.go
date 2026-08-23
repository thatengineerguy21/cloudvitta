package service

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/shopspring/decimal"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
)

// CalculateRequest specifies the input parameters for a composite workload calculation.
type CalculateRequest struct {
	Region        string
	Currency      string
	StrictFamily  bool
	Compute       *domain.ComputeAttributes
	Storage       *domain.StorageAttributes
	Network       *domain.NetworkAttributes
	DatabaseRDBMS *domain.DatabaseRDBMSAttributes
	DatabaseNoSQL *domain.DatabaseNoSQLAttributes
	Kubernetes    *domain.KubernetesAttributes
	Serverless    *ServerlessWorkload
}

// CalculateCategoryResult contains the matched SKU and normalized hourly cost for a single category.
type CalculateCategoryResult struct {
	SkuID               string
	MatchQuality        string
	MatchDeltaPct       float64
	MissingAttributes   []string
	Stale               bool
	NormalizedHourlyUSD decimal.Decimal
}

// CalculateProviderResult holds the per-provider aggregated calculation outcome.
type CalculateProviderResult struct {
	Provider                        string
	Categories                      map[string]CalculateCategoryResult
	TotalNormalizedHourlyUSD        *decimal.Decimal
	PartialTotalNormalizedHourlyUSD *decimal.Decimal
	Partial                         bool
}

// CalculateWarning represents a warning entry explaining omissions or system constraints.
type CalculateWarning struct {
	Provider string
	Code     string
	Message  string
}

// CalculateResult contains the complete composite calculation response.
type CalculateResult struct {
	Results  []CalculateProviderResult
	Warnings []CalculateWarning
}

// CalculateCategoryTargetBuilder extracts a MatchTarget from CalculateRequest if present.
type CalculateCategoryTargetBuilder func(req CalculateRequest) (MatchTarget, bool)

type calculateCategoryEntry struct {
	category    string
	buildTarget CalculateCategoryTargetBuilder
}

var calculateCategoryRegistry []calculateCategoryEntry

// RegisterCalculateCategory registers a category and its MatchTarget builder for composite workload calculations.
func RegisterCalculateCategory(category string, builder CalculateCategoryTargetBuilder) {
	calculateCategoryRegistry = append(calculateCategoryRegistry, calculateCategoryEntry{
		category:    category,
		buildTarget: builder,
	})
}

func init() {
	RegisterCalculateCategory("compute", func(req CalculateRequest) (MatchTarget, bool) {
		if req.Compute == nil {
			return MatchTarget{}, false
		}
		return MatchTarget{
			VCPU:         req.Compute.VCPU,
			RAMGB:        req.Compute.RAMGB,
			Family:       req.Compute.Family,
			StrictFamily: req.StrictFamily,
			Category:     "compute",
		}, true
	})

	RegisterCalculateCategory("storage", func(req CalculateRequest) (MatchTarget, bool) {
		if req.Storage == nil {
			return MatchTarget{}, false
		}
		return MatchTarget{
			SizeGB:       req.Storage.SizeGB,
			StorageClass: req.Storage.StorageClass,
			Category:     "storage",
		}, true
	})

	RegisterCalculateCategory("network", func(req CalculateRequest) (MatchTarget, bool) {
		if req.Network == nil {
			return MatchTarget{}, false
		}
		return MatchTarget{
			EgressGB:     req.Network.EgressGB,
			TransferType: req.Network.TransferType,
			Category:     "network",
		}, true
	})

	RegisterCalculateCategory("database_rdbms", func(req CalculateRequest) (MatchTarget, bool) {
		if req.DatabaseRDBMS == nil {
			return MatchTarget{}, false
		}
		return MatchTarget{
			Engine:            req.DatabaseRDBMS.Engine,
			VCPU:              req.DatabaseRDBMS.VCPU,
			RAMGB:             req.DatabaseRDBMS.RAMGB,
			DatabaseStorageGB: req.DatabaseRDBMS.StorageGB,
			DatabaseIOPS:      req.DatabaseRDBMS.IOPS,
			MultiAZ:           req.DatabaseRDBMS.MultiAZ,
			StorageFamily:     req.DatabaseRDBMS.StorageFamily,
			Category:          "database_rdbms",
		}, true
	})

	RegisterCalculateCategory("database_nosql", func(req CalculateRequest) (MatchTarget, bool) {
		if req.DatabaseNoSQL == nil {
			return MatchTarget{}, false
		}
		return MatchTarget{
			DataModel:        req.DatabaseNoSQL.DataModel,
			PricingMode:      req.DatabaseNoSQL.PricingMode,
			ReadUnits:        req.DatabaseNoSQL.ReadUnits,
			WriteUnits:       req.DatabaseNoSQL.WriteUnits,
			NoSQLStorageGB:   req.DatabaseNoSQL.StorageGB,
			StorageClass:     req.DatabaseNoSQL.StorageClass,
			NoSQLMultiRegion: req.DatabaseNoSQL.MultiRegion,
			Category:         "database_nosql",
		}, true
	})

	RegisterCalculateCategory("kubernetes", func(req CalculateRequest) (MatchTarget, bool) {
		if req.Kubernetes == nil {
			return MatchTarget{}, false
		}
		return MatchTarget{
			KubernetesTier:  req.Kubernetes.Tier,
			ClusterTopology: req.Kubernetes.ClusterTopology,
			Category:        "kubernetes",
		}, true
	})

	RegisterCalculateCategory("serverless", func(req CalculateRequest) (MatchTarget, bool) {
		if req.Serverless == nil {
			return MatchTarget{}, false
		}
		return MatchTarget{
			ServerlessWorkload: *req.Serverless,
			Category:           "serverless",
		}, true
	})
}

// ResolveAliasedField resolves an alias pointer and canonical pointer into a single canonical value,
// rejecting requests where both fields are provided with conflicting values.
func ResolveAliasedField[T any](alias, canonical *T, aliasName, canonicalName string) (*T, error) {
	switch {
	case alias != nil && canonical != nil:
		if !reflect.DeepEqual(alias, canonical) {
			return nil, fmt.Errorf("%w: both '%s' and '%s' were provided with conflicting values; provide only one when values differ", ErrConflictingFields, aliasName, canonicalName)
		}
		return canonical, nil
	case canonical != nil:
		return canonical, nil
	default:
		return alias, nil
	}
}

// Calculate orchestrates per-category matching and pricing across cloud providers,
// computing a composite normalized hourly total server-side while strictly enforcing
// the ADR 0022 honesty contract for partial provider totals.
func (s *PricingService) Calculate(ctx context.Context, req CalculateRequest) (*CalculateResult, error) {
	type requestedCategory struct {
		name   string
		target MatchTarget
	}

	var requested []requestedCategory
	for _, entry := range calculateCategoryRegistry {
		if target, ok := entry.buildTarget(req); ok {
			requested = append(requested, requestedCategory{
				name:   entry.category,
				target: target,
			})
		}
	}

	if len(requested) == 0 {
		return nil, ErrNoCategoriesRequested
	}

	region := req.Region
	if region == "" {
		region = "us-east"
	}

	var warnings []CalculateWarning

	if req.Currency != "" && req.Currency != "USD" {
		warnings = append(warnings, CalculateWarning{
			Provider: "system",
			Code:     "non_usd_currency_unsupported",
			Message:  "Only USD is currently supported. Returning results in USD.",
		})
	}

	providers := SupportedProviders()
	var results []CalculateProviderResult
	var totalProviderFailures int

	for _, prov := range providers {
		categoriesMap := make(map[string]CalculateCategoryResult)
		var providerTotal decimal.Decimal
		var categoryErrors int
		var matchedCategories int

		for _, item := range requested {
			category := item.name
			target := item.target

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
				case errors.Is(err, ErrArchitectureUnsupported):
					warnings = append(warnings, CalculateWarning{
						Provider: prov,
						Code:     "architecture_unsupported_excluded",
						Message:  "Provider does not offer the requested CPU architecture for serverless compute.",
					})
				case errors.Is(err, ErrNoMatchFound):
					warnings = append(warnings, CalculateWarning{
						Provider: prov,
						Code:     "no_match",
						Message:  "No " + category + " SKU matched the requested spec within acceptable thresholds.",
					})
				default:
					categoryErrors++
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

			if len(catResult.Warnings) > 0 {
				warnings = append(warnings, catResult.Warnings...)
			}

			categoriesMap[category] = CalculateCategoryResult{
				SkuID:               catResult.MatchResult.Observation.SkuID,
				MatchQuality:        catResult.MatchResult.MatchQuality,
				MatchDeltaPct:       catResult.MatchResult.MatchDeltaPct,
				MissingAttributes:   catResult.MatchResult.MissingAttributes,
				Stale:               catResult.Stale,
				NormalizedHourlyUSD: catResult.HourlyCost,
			}
			providerTotal = providerTotal.Add(catResult.HourlyCost)
			matchedCategories++
		}

		if categoryErrors == len(requested) {
			totalProviderFailures++
		}

		if matchedCategories == 0 {
			continue
		}

		if matchedCategories == len(requested) {
			totalCopy := providerTotal
			results = append(results, CalculateProviderResult{
				Provider:                        prov,
				Categories:                      categoriesMap,
				TotalNormalizedHourlyUSD:        &totalCopy,
				PartialTotalNormalizedHourlyUSD: nil,
				Partial:                         false,
			})
		} else {
			partialCopy := providerTotal
			results = append(results, CalculateProviderResult{
				Provider:                        prov,
				Categories:                      categoriesMap,
				TotalNormalizedHourlyUSD:        nil,
				PartialTotalNormalizedHourlyUSD: &partialCopy,
				Partial:                         true,
			})
		}
	}

	if len(results) == 0 && totalProviderFailures == len(providers) {
		return nil, ErrProviderUnavailable
	}

	return &CalculateResult{
		Results:  results,
		Warnings: warnings,
	}, nil
}

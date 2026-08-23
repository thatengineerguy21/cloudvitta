package service

import (
	"context"
	"errors"

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

// Calculate orchestrates per-category matching and pricing across cloud providers,
// computing a composite normalized hourly total server-side while strictly enforcing
// the ADR 0022 honesty contract for partial provider totals.
func (s *PricingService) Calculate(ctx context.Context, req CalculateRequest) (*CalculateResult, error) {
	var requestedCategories []string
	if req.Compute != nil {
		requestedCategories = append(requestedCategories, "compute")
	}
	if req.Storage != nil {
		requestedCategories = append(requestedCategories, "storage")
	}
	if req.Network != nil {
		requestedCategories = append(requestedCategories, "network")
	}
	if req.DatabaseRDBMS != nil {
		requestedCategories = append(requestedCategories, "database_rdbms")
	}
	if req.DatabaseNoSQL != nil {
		requestedCategories = append(requestedCategories, "database_nosql")
	}
	if req.Kubernetes != nil {
		requestedCategories = append(requestedCategories, "kubernetes")
	}
	if req.Serverless != nil {
		requestedCategories = append(requestedCategories, "serverless")
	}

	if len(requestedCategories) == 0 {
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
			Code:     "currency_conversion_not_yet_supported",
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

		for _, category := range requestedCategories {
			var target MatchTarget
			switch category {
			case "compute":
				target = MatchTarget{
					VCPU:         req.Compute.VCPU,
					RAMGB:        req.Compute.RAMGB,
					Family:       req.Compute.Family,
					StrictFamily: req.StrictFamily,
					Category:     "compute",
				}
			case "storage":
				target = MatchTarget{
					SizeGB:       req.Storage.SizeGB,
					StorageClass: req.Storage.StorageClass,
					Category:     "storage",
				}
			case "network":
				target = MatchTarget{
					EgressGB:     req.Network.EgressGB,
					TransferType: req.Network.TransferType,
					Category:     "network",
				}
			case "database_rdbms":
				target = MatchTarget{
					Engine:            req.DatabaseRDBMS.Engine,
					VCPU:              req.DatabaseRDBMS.VCPU,
					RAMGB:             req.DatabaseRDBMS.RAMGB,
					DatabaseStorageGB: req.DatabaseRDBMS.StorageGB,
					DatabaseIOPS:      req.DatabaseRDBMS.IOPS,
					MultiAZ:           req.DatabaseRDBMS.MultiAZ,
					StorageFamily:     req.DatabaseRDBMS.StorageFamily,
					Category:          "database_rdbms",
				}
			case "database_nosql":
				target = MatchTarget{
					DataModel:        req.DatabaseNoSQL.DataModel,
					PricingMode:      req.DatabaseNoSQL.PricingMode,
					ReadUnits:        req.DatabaseNoSQL.ReadUnits,
					WriteUnits:       req.DatabaseNoSQL.WriteUnits,
					NoSQLStorageGB:   req.DatabaseNoSQL.StorageGB,
					StorageClass:     req.DatabaseNoSQL.StorageClass,
					NoSQLMultiRegion: req.DatabaseNoSQL.MultiRegion,
					Category:         "database_nosql",
				}
			case "kubernetes":
				target = MatchTarget{
					KubernetesTier:  req.Kubernetes.Tier,
					ClusterTopology: req.Kubernetes.ClusterTopology,
					Category:        "kubernetes",
				}
			case "serverless":
				target = MatchTarget{
					ServerlessWorkload: *req.Serverless,
					Category:           "serverless",
				}
			}

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

		if categoryErrors == len(requestedCategories) {
			totalProviderFailures++
		}

		if matchedCategories == 0 {
			continue
		}

		if matchedCategories == len(requestedCategories) {
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

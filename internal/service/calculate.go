package service

import (
	"context"
	"fmt"

	"github.com/shopspring/decimal"
)

// CalculateComputeTarget defines compute parameters for composite calculation.
type CalculateComputeTarget struct {
	VCPU         float64
	RAMGB        float64
	Family       string
	StrictFamily bool
}

// CalculateStorageTarget defines storage parameters for composite calculation.
type CalculateStorageTarget struct {
	SizeGB       decimal.Decimal
	StorageClass string
}

// CalculateNetworkTarget defines network parameters for composite calculation.
type CalculateNetworkTarget struct {
	EgressGB     decimal.Decimal
	TransferType string
}

// CalculateRequest specifies the input parameters for a composite workload calculation.
type CalculateRequest struct {
	Region   string
	Currency string
	Compute  *CalculateComputeTarget
	Storage  *CalculateStorageTarget
	Network  *CalculateNetworkTarget
}

// CalculateCategoryResult contains the matched SKU and normalized hourly cost for a single category.
type CalculateCategoryResult struct {
	SkuID               string          `json:"sku_id"`
	MatchQuality        string          `json:"match_quality"`
	NormalizedHourlyUSD decimal.Decimal `json:"normalized_hourly_usd"`
}

// CalculateProviderResult holds the per-provider aggregated calculation outcome.
type CalculateProviderResult struct {
	Provider                        string                             `json:"provider"`
	Categories                      map[string]CalculateCategoryResult `json:"categories"`
	TotalNormalizedHourlyUSD        *decimal.Decimal                   `json:"total_normalized_hourly_usd,omitempty"`
	PartialTotalNormalizedHourlyUSD *decimal.Decimal                   `json:"partial_total_normalized_hourly_usd,omitempty"`
	Partial                         bool                               `json:"partial"`
}

// CalculateWarning represents a warning entry explaining omissions or system constraints.
type CalculateWarning struct {
	Provider string `json:"provider"`
	Code     string `json:"code"`
	Message  string `json:"message"`
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

	if len(requestedCategories) == 0 {
		return nil, ErrNoCategoriesRequested
	}

	region := req.Region
	if region == "" {
		region = "us-east"
	}

	var warnings []CalculateWarning

	// Stage 3 provider warnings
	stage3Providers := []string{"oracle", "ibm", "alibaba", "digitalocean"}
	for _, prov := range stage3Providers {
		warnings = append(warnings, CalculateWarning{
			Provider: prov,
			Code:     "not_yet_ingested",
			Message:  fmt.Sprintf("%s ingestion lands in stage 3.", formatProviderDisplayName(prov)),
		})
	}

	if req.Currency != "" && req.Currency != "USD" {
		warnings = append(warnings, CalculateWarning{
			Provider: "system",
			Code:     "currency_conversion_not_yet_supported",
			Message:  "Currency conversion is not yet supported. Prices are returned in USD.",
		})
	}

	providers := []string{"aws", "azure", "gcp"}
	var results []CalculateProviderResult
	var totalProviderFailures int

	for _, prov := range providers {
		categoriesMap := make(map[string]CalculateCategoryResult)
		var providerTotal decimal.Decimal
		var categoryErrors int
		var matchedCategories int

		for _, category := range requestedCategories {
			if !IsProviderCategorySupported(prov, category) {
				warnings = append(warnings, CalculateWarning{
					Provider: prov,
					Code:     "category_not_supported",
					Message:  fmt.Sprintf("%s category is not supported by %s", category, prov),
				})
				continue
			}

			obsList, err := s.GetPrices(ctx, prov, category, region)
			if err != nil {
				categoryErrors++
				warnings = append(warnings, CalculateWarning{
					Provider: prov,
					Code:     "fetch_failed",
					Message:  err.Error(),
				})
				continue
			}

			if len(obsList) == 0 {
				warnings = append(warnings, CalculateWarning{
					Provider: prov,
					Code:     "no_data_available",
					Message:  fmt.Sprintf("No %s pricing data available for this region.", category),
				})
				continue
			}

			switch category {
			case "compute":
				target := MatchTarget{
					VCPU:         req.Compute.VCPU,
					RAMGB:        req.Compute.RAMGB,
					Family:       req.Compute.Family,
					StrictFamily: req.Compute.StrictFamily,
					Category:     "compute",
				}
				matchResult := MatchObservations(
					ComputeScorer{}, obsList, target, ThresholdsForCategory("compute"),
				)
				if matchResult == nil {
					warnings = append(warnings, CalculateWarning{
						Provider: prov,
						Code:     "no_match",
						Message:  "No compute SKU matched the requested spec within acceptable thresholds.",
					})
					continue
				}

				hourlyCost := matchResult.Observation.PriceAmount
				categoriesMap["compute"] = CalculateCategoryResult{
					SkuID:               matchResult.Observation.SkuID,
					MatchQuality:        matchResult.MatchQuality,
					NormalizedHourlyUSD: hourlyCost,
				}
				providerTotal = providerTotal.Add(hourlyCost)
				matchedCategories++

			case "storage":
				sizeGB := decimal.NewFromInt(1)
				if req.Storage.SizeGB.GreaterThan(decimal.Zero) {
					sizeGB = req.Storage.SizeGB
				}
				sizeF, _ := sizeGB.Float64()
				target := MatchTarget{
					SizeGB:       sizeF,
					StorageClass: req.Storage.StorageClass,
					Category:     "storage",
				}
				matchResult := MatchObservations(
					StorageScorer{}, obsList, target, ThresholdsForCategory("storage"),
				)
				if matchResult == nil {
					warnings = append(warnings, CalculateWarning{
						Provider: prov,
						Code:     "no_match",
						Message:  "No storage SKU matched the requested spec within acceptable thresholds.",
					})
					continue
				}

				hourlyCost := CalculateStorageHourlyCost(matchResult.Observation.PriceAmount, sizeGB)
				categoriesMap["storage"] = CalculateCategoryResult{
					SkuID:               matchResult.Observation.SkuID,
					MatchQuality:        matchResult.MatchQuality,
					NormalizedHourlyUSD: hourlyCost,
				}
				providerTotal = providerTotal.Add(hourlyCost)
				matchedCategories++

			case "network":
				egressGB := decimal.NewFromInt(1)
				if req.Network.EgressGB.GreaterThan(decimal.Zero) {
					egressGB = req.Network.EgressGB
				}
				egressF, _ := egressGB.Float64()
				target := MatchTarget{
					EgressGB:     egressF,
					TransferType: req.Network.TransferType,
					Category:     "network",
				}
				matchResult := MatchObservations(
					NetworkScorer{}, obsList, target, ThresholdsForCategory("network"),
				)
				if matchResult == nil {
					warnings = append(warnings, CalculateWarning{
						Provider: prov,
						Code:     "no_match",
						Message:  "No network SKU matched the requested spec within acceptable thresholds.",
					})
					continue
				}

				hourlyCost := CalculateNetworkHourlyCost(matchResult.Observation.PriceAmount, egressGB)
				categoriesMap["network"] = CalculateCategoryResult{
					SkuID:               matchResult.Observation.SkuID,
					MatchQuality:        matchResult.MatchQuality,
					NormalizedHourlyUSD: hourlyCost,
				}
				providerTotal = providerTotal.Add(hourlyCost)
				matchedCategories++
			}
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

func formatProviderDisplayName(provider string) string {
	switch provider {
	case "oracle":
		return "Oracle OCI"
	case "ibm":
		return "IBM Cloud"
	case "alibaba":
		return "Alibaba Cloud"
	case "digitalocean":
		return "DigitalOcean"
	default:
		return provider
	}
}

package gcp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/regionmap"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/serverlessarchmap"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/serverlessunitmap"
	"github.com/thatengineerguy21/CloudVitta/internal/quarantine"
)

// normalizeServerlessSKU normalizes a Google Cloud Functions / Cloud Run functions pricing SKU.
func normalizeServerlessSKU(sku gcpSKU, category string, fetchedAt time.Time, sink quarantine.Sink) ([]domain.PriceObservation, error) {
	if len(sku.PricingInfo) == 0 || len(sku.PricingInfo[0].PricingExpression.TieredRates) == 0 {
		return nil, nil
	}

	var priceAmount decimal.Decimal
	var unitPrice gcpUnitPrice
	for _, rate := range sku.PricingInfo[0].PricingExpression.TieredRates {
		ratePrice, err := extractUnitPrice(rate.UnitPrice)
		if err != nil {
			return nil, fmt.Errorf("gcp normalize sku %s: %w", sku.SkuID, err)
		}
		if !ratePrice.IsZero() {
			priceAmount = ratePrice
			unitPrice = rate.UnitPrice
			break
		}
	}
	if priceAmount.IsZero() {
		return nil, nil
	}

	desc := sku.Description
	resGroup := sku.Category.ResourceGroup
	descLower := strings.ToLower(desc)
	resGroupLower := strings.ToLower(resGroup)

	var componentType string
	switch {
	case strings.Contains(descLower, "invocation") || strings.Contains(resGroupLower, "invocation") || strings.Contains(descLower, "request"):
		componentType = domain.ComponentTypeRequestFee
	case strings.Contains(descLower, "cpu time") || strings.Contains(descLower, "cpu") || strings.Contains(resGroupLower, "cpu") ||
		strings.Contains(descLower, "ghz") || strings.Contains(descLower, "vcpu"):
		componentType = domain.ComponentTypeDurationFeeCPU
	case strings.Contains(descLower, "memory time") || strings.Contains(descLower, "memory") || strings.Contains(resGroupLower, "memory") ||
		strings.Contains(descLower, "gb-second") || strings.Contains(descLower, "gib-second") || strings.Contains(descLower, "giby.s") || strings.Contains(descLower, "gb.s"):
		componentType = domain.ComponentTypeDurationFeeMemory
	case strings.Contains(descLower, "execution time") || strings.Contains(descLower, "time") || strings.Contains(resGroupLower, "time"):
		// Generic time meter fallback: if memory is mentioned, treat as memory duration; otherwise CPU duration
		if strings.Contains(descLower, "memory") {
			componentType = domain.ComponentTypeDurationFeeMemory
		} else {
			componentType = domain.ComponentTypeDurationFeeCPU
		}
	default:
		return nil, nil
	}

	tier := domain.ServerlessTierConsumption
	if strings.Contains(desc, "1st Gen") || strings.Contains(resGroup, "1stGen") {
		tier = domain.ServerlessTier1stGen
	} else if strings.Contains(desc, "2nd Gen") || strings.Contains(resGroup, "2ndGen") {
		tier = domain.ServerlessTier2ndGen
	}

	arch, err := serverlessarchmap.MapGCPArchitecture(desc)
	if err != nil {
		if errors.Is(err, serverlessarchmap.ErrUnmappedArchitecture) {
			if sink != nil {
				_ = sink.Record(context.Background(), quarantine.UnmappedItem{
					Provider:   "gcp",
					Category:   category,
					Kind:       "serverless_architecture",
					RawValue:   desc,
					SkuID:      sku.SkuID,
					ObservedAt: fetchedAt,
				})
			}
			slog.Warn("gcp normalize: skipping SKU due to unmapped serverless architecture", "sku", sku.SkuID, "description", desc)
			return nil, nil
		}
		return nil, fmt.Errorf("gcp normalize sku %s: %w", sku.SkuID, err)
	}

	regions := sku.ServiceRegions
	if len(regions) == 0 {
		regions = []string{"global"}
	}

	currency := unitPrice.CurrencyCode
	if currency == "" {
		currency = "USD"
	}

	rawUnit := sku.PricingInfo[0].PricingExpression.UsageUnit
	canonicalUnit, err := serverlessunitmap.MapGCPUnit(rawUnit, componentType)
	if err != nil && desc != "" {
		canonicalUnit, err = serverlessunitmap.MapGCPUnit(desc, componentType)
	}
	if err != nil {
		if errors.Is(err, serverlessunitmap.ErrUnmappedUnit) {
			if sink != nil {
				_ = sink.Record(context.Background(), quarantine.UnmappedItem{
					Provider:   "gcp",
					Category:   category,
					Kind:       "serverless_unit",
					RawValue:   rawUnit + " " + desc,
					SkuID:      sku.SkuID,
					ObservedAt: fetchedAt,
				})
			}
			slog.Warn("gcp normalize: skipping SKU due to unmapped serverless unit", "sku", sku.SkuID, "unit", rawUnit, "description", desc)
			return nil, nil
		}
		return nil, fmt.Errorf("gcp normalize sku %s: %w", sku.SkuID, err)
	}

	var results []domain.PriceObservation
	for _, region := range regions {
		regionGroup, err := regionmap.MapGCPRegion(region)
		if err != nil {
			if errors.Is(err, regionmap.ErrUnmappedRegion) {
				if sink != nil {
					_ = sink.Record(context.Background(), quarantine.UnmappedItem{
						Provider:   "gcp",
						Category:   category,
						Kind:       "region",
						RawValue:   region,
						SkuID:      sku.SkuID,
						ObservedAt: fetchedAt,
					})
				}
				slog.Warn("gcp normalize: skipping SKU due to unmapped region", "sku", sku.SkuID, "region", region)
				continue
			}
			return nil, fmt.Errorf("gcp normalize sku %s: %w", sku.SkuID, err)
		}

		if !regionmap.IsTargetRegion("gcp", region) {
			continue
		}

		results = append(results, domain.PriceObservation{
			Provider:        "gcp",
			ServiceCategory: category,
			SkuID:           sku.SkuID,
			DisplayName:     desc,
			Region:          region,
			RegionGroup:     regionGroup,
			Unit:            rawUnit,
			PriceAmount:     priceAmount,
			PriceCurrency:   currency,
			PricingModel:    "OnDemand",
			ServerlessRateAttributes: domain.ServerlessRateAttributes{
				Architecture:  arch,
				Tier:          tier,
				ComponentType: componentType,
				Unit:          canonicalUnit,
			},
			FetchedAt: fetchedAt,
		})
	}

	return results, nil
}

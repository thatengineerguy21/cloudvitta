package gcp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/regionmap"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/serverlessarchmap"
	"github.com/thatengineerguy21/CloudVitta/internal/quarantine"
)

// normalizeServerlessSKU normalizes a Google Cloud Functions / Cloud Run functions pricing SKU.
func normalizeServerlessSKU(sku gcpSKU, category string, fetchedAt time.Time, sink quarantine.Sink) ([]domain.PriceObservation, error) {
	if len(sku.PricingInfo) == 0 || len(sku.PricingInfo[0].PricingExpression.TieredRates) == 0 {
		return nil, nil
	}

	unitPrice := sku.PricingInfo[0].PricingExpression.TieredRates[0].UnitPrice
	priceAmount, err := extractUnitPrice(unitPrice)
	if err != nil {
		return nil, fmt.Errorf("gcp normalize sku %s: %w", sku.SkuID, err)
	}
	if priceAmount.IsZero() {
		return nil, nil
	}

	desc := sku.Description
	resGroup := sku.Category.ResourceGroup

	var componentType string
	switch {
	case strings.Contains(desc, "Invocation") || strings.Contains(resGroup, "Invocation"):
		componentType = domain.ComponentTypeRequestFee
	case strings.Contains(desc, "Time") || strings.Contains(desc, "Second") || strings.Contains(desc, "Duration") ||
		strings.Contains(resGroup, "Time") || strings.Contains(resGroup, "CPU") || strings.Contains(resGroup, "Memory"):
		componentType = domain.ComponentTypeDurationFee
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

	unit := sku.PricingInfo[0].PricingExpression.UsageUnit

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

		results = append(results, domain.PriceObservation{
			Provider:        "gcp",
			ServiceCategory: category,
			SkuID:           sku.SkuID,
			DisplayName:     desc,
			Region:          region,
			RegionGroup:     regionGroup,
			Unit:            unit,
			PriceAmount:     priceAmount,
			PriceCurrency:   currency,
			PricingModel:    "OnDemand",
			ServerlessRateAttributes: domain.ServerlessRateAttributes{
				Architecture:  arch,
				Tier:          tier,
				ComponentType: componentType,
			},
			FetchedAt: fetchedAt,
		})
	}

	return results, nil
}

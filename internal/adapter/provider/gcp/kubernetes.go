package gcp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/kubernetestieremap"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/regionmap"
	"github.com/thatengineerguy21/CloudVitta/internal/quarantine"
)

// normalizeKubernetesSKU normalizes a Google Kubernetes Engine (GKE) cluster management fee SKU.
func normalizeKubernetesSKU(sku gcpSKU, category string, fetchedAt time.Time, sink quarantine.Sink) ([]domain.PriceObservation, error) {
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

	tier, err := kubernetestieremap.MapGCPTier(sku.Description)
	if err != nil {
		if errors.Is(err, kubernetestieremap.ErrUnmappedTier) {
			if sink != nil {
				_ = sink.Record(context.Background(), quarantine.UnmappedItem{
					Provider:   "gcp",
					Category:   category,
					Kind:       "kubernetes_tier",
					RawValue:   sku.Description,
					SkuID:      sku.SkuID,
					ObservedAt: fetchedAt,
				})
			}
			slog.Warn("gcp normalize: skipping SKU due to unmapped kubernetes tier", "sku", sku.SkuID, "description", sku.Description)
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
	if unit == "" || strings.EqualFold(unit, "h") || strings.EqualFold(unit, "hour") || strings.EqualFold(unit, "hrs") {
		unit = "hour"
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

		results = append(results, domain.PriceObservation{
			Provider:        "gcp",
			ServiceCategory: category,
			SkuID:           sku.SkuID,
			DisplayName:     sku.Description,
			Region:          region,
			RegionGroup:     regionGroup,
			Unit:            unit,
			PriceAmount:     priceAmount,
			PriceCurrency:   currency,
			PricingModel:    "OnDemand",
			KubernetesAttributes: domain.KubernetesAttributes{
				Tier: tier,
			},
			FetchedAt: fetchedAt,
		})
	}
	return results, nil
}

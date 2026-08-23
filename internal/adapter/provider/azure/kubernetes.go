package azure

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/kubernetestieremap"
	"github.com/thatengineerguy21/CloudVitta/internal/quarantine"
)

// isAzureKubernetesProduct reports whether an Azure item belongs to Azure Kubernetes Service.
func isAzureKubernetesProduct(item azureItem) bool {
	return item.ServiceName == "Azure Kubernetes Service" || strings.Contains(strings.ToLower(item.ProductName), "kubernetes")
}

// normalizeAzureKubernetesItem normalizes an Azure AKS control plane retail pricing item.
func normalizeAzureKubernetesItem(item azureItem, category, region, regionGroup, skuID string, priceAmount decimal.Decimal, fetchedAt time.Time, sink quarantine.Sink) (*domain.PriceObservation, error) {
	if !isAzureKubernetesProduct(item) {
		return nil, nil
	}

	tierLookupKey := item.MeterName
	if tierLookupKey == "" {
		tierLookupKey = item.SkuName
	}

	tier, err := kubernetestieremap.MapAzureTier(tierLookupKey)
	if err != nil && item.SkuName != "" && item.SkuName != tierLookupKey {
		tier, err = kubernetestieremap.MapAzureTier(item.SkuName)
	}
	if err != nil {
		if errors.Is(err, kubernetestieremap.ErrUnmappedTier) {
			if sink != nil {
				_ = sink.Record(context.Background(), quarantine.UnmappedItem{
					Provider:   "azure",
					Category:   category,
					Kind:       "kubernetes_tier",
					RawValue:   item.SkuName + " " + item.MeterName,
					SkuID:      skuID,
					ObservedAt: fetchedAt,
				})
			}
			slog.Warn("azure normalize: skipping SKU due to unmapped kubernetes tier", "sku", skuID, "sku_name", item.SkuName, "meter_name", item.MeterName)
			return nil, nil
		}
		return nil, fmt.Errorf("azure normalize sku %s: %w", item.SkuID, err)
	}

	displayName := item.ProductName
	if displayName == "" {
		displayName = item.MeterName
	}

	unit := item.UnitOfMeasure
	if unit == "1 Hour" || unit == "1 hour" {
		unit = "Hrs"
	}

	return &domain.PriceObservation{
		Provider:        "azure",
		ServiceCategory: category,
		SkuID:           skuID,
		DisplayName:     displayName,
		Region:          region,
		RegionGroup:     regionGroup,
		Unit:            unit,
		PriceAmount:     priceAmount,
		PriceCurrency:   item.CurrencyCode,
		PricingModel:    "OnDemand",
		KubernetesAttributes: domain.KubernetesAttributes{
			Tier: tier,
		},
		FetchedAt: fetchedAt,
	}, nil
}

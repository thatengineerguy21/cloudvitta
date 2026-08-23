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
	"github.com/thatengineerguy21/CloudVitta/internal/matching/serverlessarchmap"
	"github.com/thatengineerguy21/CloudVitta/internal/quarantine"
)

// isAzureServerlessProduct reports whether an Azure item belongs to Azure Functions serverless compute.
func isAzureServerlessProduct(item azureItem) bool {
	if strings.Contains(item.ProductName, "Premium") || strings.Contains(item.ServiceName, "Premium") {
		return false
	}
	if item.ServiceName == "Functions" || item.ServiceName == "Azure Functions" || item.ProductName == "Flex Consumption" {
		return true
	}
	if strings.Contains(item.ProductName, "Functions") || strings.Contains(item.MeterName, "Executions") {
		return true
	}
	return false
}

// normalizeAzureServerlessItem normalizes an Azure Functions retail pricing item into a domain PriceObservation.
func normalizeAzureServerlessItem(item azureItem, category, region, regionGroup, skuID string, priceAmount decimal.Decimal, fetchedAt time.Time, sink quarantine.Sink) (*domain.PriceObservation, error) {
	if !isAzureServerlessProduct(item) {
		return nil, nil
	}

	meterName := item.MeterName
	skuName := item.SkuName
	productName := item.ProductName

	var componentType string
	switch {
	case strings.Contains(meterName, "Executions") || strings.Contains(skuName, "Executions"):
		componentType = domain.ComponentTypeRequestFee
	case strings.Contains(meterName, "Execution Time") || strings.Contains(meterName, "Execution") || strings.Contains(meterName, "Time"):
		componentType = domain.ComponentTypeDurationFee
	default:
		return nil, nil
	}

	tier := domain.ServerlessTierConsumption
	if strings.Contains(productName, "Flex") || strings.Contains(skuName, "Flex") || strings.Contains(meterName, "Flex") || strings.Contains(skuName, "On Demand") {
		tier = domain.ServerlessTierFlexConsumption
	}

	archLookupKey := meterName
	if archLookupKey == "" {
		archLookupKey = skuName
	}

	arch, err := serverlessarchmap.MapAzureArchitecture(archLookupKey)
	if err != nil && skuName != "" && skuName != archLookupKey {
		arch, err = serverlessarchmap.MapAzureArchitecture(skuName)
	}
	if err != nil {
		if errors.Is(err, serverlessarchmap.ErrUnmappedArchitecture) {
			if sink != nil {
				_ = sink.Record(context.Background(), quarantine.UnmappedItem{
					Provider:   "azure",
					Category:   category,
					Kind:       "serverless_architecture",
					RawValue:   skuName + " " + meterName,
					SkuID:      skuID,
					ObservedAt: fetchedAt,
				})
			}
			slog.Warn("azure normalize: skipping SKU due to unmapped serverless architecture", "sku", skuID, "sku_name", skuName, "meter_name", meterName)
			return nil, nil
		}
		return nil, fmt.Errorf("azure normalize sku %s: %w", item.SkuID, err)
	}

	displayName := item.ProductName
	if displayName == "" {
		displayName = item.MeterName
	}

	unit := item.UnitOfMeasure

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
		ServerlessRateAttributes: domain.ServerlessRateAttributes{
			Architecture:  arch,
			Tier:          tier,
			ComponentType: componentType,
		},
		FetchedAt: fetchedAt,
	}, nil
}

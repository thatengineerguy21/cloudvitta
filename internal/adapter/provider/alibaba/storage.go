package alibaba

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/regionmap"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/storageclassmap"
)

// normalizeStorageItem converts an Alibaba Cloud storage item into normalized PriceObservation records.
func (n *computeNormalizer) normalizeStorageItem(item InstanceTypeItem) ([]domain.PriceObservation, error) {
	rawClass := item.InstanceTypeID
	if rawClass == "" {
		rawClass = item.ProductCode
	}

	storageClass, err := storageclassmap.MapAlibabaStorageClass(rawClass)
	if err != nil {
		if errors.Is(err, storageclassmap.ErrUnmappedStorageClass) {
			n.recordQuarantine("storage_class", rawClass, item.InstanceTypeID, "storage")
			slog.Warn("alibaba normalize: skipping item due to unmapped storage class", "instance_type_id", item.InstanceTypeID, "storage_class", rawClass)
			return nil, nil
		}
		return nil, fmt.Errorf("alibaba normalize storage item %s: %w", item.InstanceTypeID, err)
	}

	priceAmount, currency := extractPriceAndCurrency(item)
	if priceAmount.IsZero() {
		return nil, nil
	}

	regions := item.Regions
	if len(regions) == 0 && item.RegionID != "" {
		regions = []string{item.RegionID}
	}

	if len(regions) == 0 {
		n.recordQuarantine("region", "missing", item.InstanceTypeID, "storage")
		slog.Warn("alibaba normalize: skipping storage SKU due to missing regions", "sku", item.InstanceTypeID)
		return nil, nil
	}

	cleanType := strings.ToUpper(strings.ReplaceAll(item.InstanceTypeID, ".", "-"))
	cleanType = strings.TrimPrefix(cleanType, "OSS-")
	skuID := fmt.Sprintf("SKU-ALI-STORAGE-%s", cleanType)
	displayName := fmt.Sprintf("Alibaba Cloud Storage %s", item.InstanceTypeID)

	var observations []domain.PriceObservation
	for _, region := range regions {
		regionGroup, err := regionmap.MapAlibabaRegion(region)
		if err != nil {
			if errors.Is(err, regionmap.ErrUnmappedRegion) {
				n.recordQuarantine("region", region, skuID, "storage")
				slog.Warn("alibaba normalize: skipping storage SKU due to unmapped region", "sku", skuID, "region", region)
				continue
			}
			return nil, fmt.Errorf("alibaba normalize storage sku %s region %s: %w", skuID, region, err)
		}

		dedupKey := fmt.Sprintf("%s:%s", skuID, region)
		if n.seen[dedupKey] {
			continue
		}
		n.seen[dedupKey] = true

		observations = append(observations, domain.PriceObservation{
			Provider:        "alibaba",
			ServiceCategory: "storage",
			SkuID:           skuID,
			DisplayName:     displayName,
			Region:          region,
			RegionGroup:     regionGroup,
			Unit:            "GB-Mo",
			PriceAmount:     priceAmount,
			PriceCurrency:   currency,
			PricingModel:    "OnDemand",
			StorageAttributes: domain.StorageAttributes{
				SizeGB:       1,
				StorageClass: storageClass,
			},
			FetchedAt: n.fetchedAt,
		})
	}

	return observations, nil
}

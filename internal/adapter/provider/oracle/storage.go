package oracle

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/regionmap"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/storageclassmap"
)

// normalizeStorageItem converts an Oracle OCI storage ProductItem into normalized PriceObservation records.
func (n *oracleNormalizer) normalizeStorageItem(item ProductItem) ([]domain.PriceObservation, error) {
	rawClass := item.DisplayName
	if rawClass == "" {
		rawClass = item.ServiceCategory
	}

	storageClass, err := storageclassmap.MapOracleStorageClass(rawClass)
	if err != nil {
		if errors.Is(err, storageclassmap.ErrUnmappedStorageClass) {
			n.recordQuarantine("storage_class", rawClass, item.PartNumber, "storage")
			slog.Warn("oracle normalize: skipping SKU due to unmapped storage class", "part_number", item.PartNumber, "storage_class", rawClass)
			return nil, nil
		}
		return nil, fmt.Errorf("oracle normalize storage part %s: %w", item.PartNumber, err)
	}

	priceAmount, err := extractPrice(item)
	if err != nil {
		return nil, fmt.Errorf("oracle normalize storage part %s: %w", item.PartNumber, err)
	}
	if priceAmount.IsZero() {
		return nil, nil
	}

	regions := item.Regions
	if len(regions) == 0 {
		n.recordQuarantine("region", "missing", item.PartNumber, "storage")
		slog.Warn("oracle normalize: skipping storage SKU due to missing regions", "part_number", item.PartNumber)
		return nil, nil
	}

	cleanPart := strings.ToUpper(strings.ReplaceAll(item.PartNumber, ".", "-"))
	skuID := fmt.Sprintf("SKU-OCI-%s", cleanPart)
	displayName := item.DisplayName
	if displayName == "" {
		displayName = fmt.Sprintf("Oracle OCI Storage %s", item.PartNumber)
	}

	var observations []domain.PriceObservation
	for _, region := range regions {
		regionGroup, err := regionmap.MapOracleRegion(region)
		if err != nil {
			if errors.Is(err, regionmap.ErrUnmappedRegion) {
				n.recordQuarantine("region", region, skuID, "storage")
				slog.Warn("oracle normalize: skipping storage SKU due to unmapped region", "sku", skuID, "region", region)
				continue
			}
			return nil, fmt.Errorf("oracle normalize storage sku %s region %s: %w", skuID, region, err)
		}

		dedupKey := fmt.Sprintf("%s:%s", skuID, region)
		if n.seen[dedupKey] {
			continue
		}
		n.seen[dedupKey] = true

		observations = append(observations, domain.PriceObservation{
			Provider:        "oracle",
			ServiceCategory: "storage",
			SkuID:           skuID,
			DisplayName:     displayName,
			Region:          region,
			RegionGroup:     regionGroup,
			Unit:            "GB-Mo",
			PriceAmount:     priceAmount,
			PriceCurrency:   "USD",
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

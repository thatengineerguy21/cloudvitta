package oracle

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/regionmap"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/transfertypemap"
)

// normalizeNetworkItem converts an Oracle OCI network ProductItem into normalized PriceObservation records.
func (n *oracleNormalizer) normalizeNetworkItem(item ProductItem) ([]domain.PriceObservation, error) {
	rawType := item.DisplayName
	if rawType == "" {
		rawType = item.ServiceCategory
	}

	transferType, err := transfertypemap.MapOracleTransferType(rawType)
	if err != nil {
		if errors.Is(err, transfertypemap.ErrUnmappedTransferType) {
			n.recordQuarantine("transfer_type", rawType, item.PartNumber, "network")
			slog.Warn("oracle normalize: skipping SKU due to unmapped transfer type", "part_number", item.PartNumber, "transfer_type", rawType)
			return nil, nil
		}
		return nil, fmt.Errorf("oracle normalize network part %s: %w", item.PartNumber, err)
	}

	priceAmount, err := extractPrice(item)
	if err != nil {
		return nil, fmt.Errorf("oracle normalize network part %s: %w", item.PartNumber, err)
	}
	if priceAmount.IsZero() {
		return nil, nil
	}

	regions := item.Regions
	if len(regions) == 0 {
		n.recordQuarantine("region", "missing", item.PartNumber, "network")
		slog.Warn("oracle normalize: skipping network SKU due to missing regions", "part_number", item.PartNumber)
		return nil, nil
	}

	cleanPart := strings.ToUpper(strings.ReplaceAll(item.PartNumber, ".", "-"))
	skuID := fmt.Sprintf("SKU-OCI-%s", cleanPart)
	displayName := item.DisplayName
	if displayName == "" {
		displayName = fmt.Sprintf("Oracle OCI Network %s", item.PartNumber)
	}

	var observations []domain.PriceObservation
	for _, region := range regions {
		regionGroup, err := regionmap.MapOracleRegion(region)
		if err != nil {
			if errors.Is(err, regionmap.ErrUnmappedRegion) {
				n.recordQuarantine("region", region, skuID, "network")
				slog.Warn("oracle normalize: skipping network SKU due to unmapped region", "sku", skuID, "region", region)
				continue
			}
			return nil, fmt.Errorf("oracle normalize network sku %s region %s: %w", skuID, region, err)
		}

		dedupKey := fmt.Sprintf("%s:%s", skuID, region)
		if n.seen[dedupKey] {
			continue
		}
		n.seen[dedupKey] = true

		observations = append(observations, domain.PriceObservation{
			Provider:        "oracle",
			ServiceCategory: "network",
			SkuID:           skuID,
			DisplayName:     displayName,
			Region:          region,
			RegionGroup:     regionGroup,
			Unit:            "GB",
			PriceAmount:     priceAmount,
			PriceCurrency:   "USD",
			PricingModel:    "OnDemand",
			NetworkAttributes: domain.NetworkAttributes{
				EgressGB:     1,
				TransferType: transferType,
			},
			FetchedAt: n.fetchedAt,
		})
	}

	return observations, nil
}

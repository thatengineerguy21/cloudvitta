package alibaba

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/regionmap"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/transfertypemap"
)

// normalizeNetworkItem converts an Alibaba Cloud network item into normalized PriceObservation records.
func (n *computeNormalizer) normalizeNetworkItem(item InstanceTypeItem) ([]domain.PriceObservation, error) {
	rawType := item.InstanceTypeID
	if rawType == "" {
		rawType = item.ProductCode
	}

	transferType, err := transfertypemap.MapAlibabaTransferType(rawType)
	if err != nil {
		if errors.Is(err, transfertypemap.ErrUnmappedTransferType) {
			n.recordQuarantine("transfer_type", rawType, item.InstanceTypeID, "network")
			slog.Warn("alibaba normalize: skipping item due to unmapped transfer type", "instance_type_id", item.InstanceTypeID, "transfer_type", rawType)
			return nil, nil
		}
		return nil, fmt.Errorf("alibaba normalize network item %s: %w", item.InstanceTypeID, err)
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
		n.recordQuarantine("region", "missing", item.InstanceTypeID, "network")
		slog.Warn("alibaba normalize: skipping network SKU due to missing regions", "sku", item.InstanceTypeID)
		return nil, nil
	}

	cleanType := strings.ToUpper(strings.ReplaceAll(item.InstanceTypeID, ".", "-"))
	skuID := fmt.Sprintf("SKU-ALI-NETWORK-%s", cleanType)
	displayName := fmt.Sprintf("Alibaba Cloud Network %s", item.InstanceTypeID)

	var observations []domain.PriceObservation
	for _, region := range regions {
		regionGroup, err := regionmap.MapAlibabaRegion(region)
		if err != nil {
			if errors.Is(err, regionmap.ErrUnmappedRegion) {
				n.recordQuarantine("region", region, skuID, "network")
				slog.Warn("alibaba normalize: skipping network SKU due to unmapped region", "sku", skuID, "region", region)
				continue
			}
			return nil, fmt.Errorf("alibaba normalize network sku %s region %s: %w", skuID, region, err)
		}

		dedupKey := fmt.Sprintf("%s:%s", skuID, region)
		if n.seen[dedupKey] {
			continue
		}
		n.seen[dedupKey] = true

		observations = append(observations, domain.PriceObservation{
			Provider:        "alibaba",
			ServiceCategory: "network",
			SkuID:           skuID,
			DisplayName:     displayName,
			Region:          region,
			RegionGroup:     regionGroup,
			Unit:            "GB",
			PriceAmount:     priceAmount,
			PriceCurrency:   currency,
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

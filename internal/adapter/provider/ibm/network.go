package ibm

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/regionmap"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/transfertypemap"
)

// normalizeNetworkMetric converts an IBM Cloud network pricing metric into normalized PriceObservation records.
func (n *computeNormalizer) normalizeNetworkMetric(
	metric PricingMetric,
	resource Resource,
	geoTags []string,
) ([]domain.PriceObservation, error) {
	rawType := metric.MetricID
	if rawType == "" {
		rawType = resource.Name
	}

	transferType, err := transfertypemap.MapIBMTransferType(rawType)
	if err != nil {
		if resource.Name != "" {
			var err2 error
			transferType, err2 = transfertypemap.MapIBMTransferType(resource.Name)
			if err2 == nil {
				err = nil
			}
		}
	}

	if err != nil {
		if errors.Is(err, transfertypemap.ErrUnmappedTransferType) {
			n.recordQuarantine("transfer_type", rawType, metric.MetricID, "network")
			slog.Warn("ibm normalize: skipping metric due to unmapped transfer type", "metric_id", metric.MetricID, "transfer_type", rawType)
			return nil, nil
		}
		return nil, fmt.Errorf("ibm normalize network metric %s: %w", metric.MetricID, err)
	}

	priceAmount, err := extractMetricPrice(metric)
	if err != nil {
		return nil, fmt.Errorf("ibm normalize network metric %s: %w", metric.MetricID, err)
	}
	if priceAmount.IsZero() {
		return nil, nil
	}

	if len(geoTags) == 0 {
		geoTags = defaultIBMRegions
	}

	cleanID := strings.ToUpper(strings.ReplaceAll(metric.MetricID, ".", "-"))
	skuID := fmt.Sprintf("SKU-IBM-%s", cleanID)

	displayName := metric.ChargeUnitName
	if displayName == "" {
		displayName = metric.MetricID
	}
	if ui, ok := resource.OverviewUI["en"]; ok && ui.DisplayName != "" {
		displayName = fmt.Sprintf("%s (%s)", ui.DisplayName, metric.MetricID)
	}

	var observations []domain.PriceObservation
	for _, region := range geoTags {
		regionGroup, err := regionmap.MapIBMRegion(region)
		if err != nil {
			if errors.Is(err, regionmap.ErrUnmappedRegion) {
				n.recordQuarantine("region", region, skuID, "network")
				slog.Warn("ibm normalize: skipping network SKU due to unmapped region", "sku", skuID, "region", region)
				continue
			}
			return nil, fmt.Errorf("ibm normalize network sku %s region %s: %w", skuID, region, err)
		}

		dedupKey := fmt.Sprintf("%s:%s", skuID, region)
		if n.seen[dedupKey] {
			continue
		}
		n.seen[dedupKey] = true

		observations = append(observations, domain.PriceObservation{
			Provider:        "ibm",
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

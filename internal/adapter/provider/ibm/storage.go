package ibm

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/regionmap"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/storageclassmap"
)

// normalizeStorageMetric converts an IBM Cloud storage pricing metric into normalized PriceObservation records.
func (n *computeNormalizer) normalizeStorageMetric(
	metric PricingMetric,
	resource Resource,
	geoTags []string,
) ([]domain.PriceObservation, error) {
	rawClass := metric.MetricID
	if rawClass == "" {
		rawClass = resource.Name
	}

	storageClass, err := storageclassmap.MapIBMStorageClass(rawClass)
	if err != nil {
		// Fall back to resource name if metric ID unmapped
		if resource.Name != "" {
			var err2 error
			storageClass, err2 = storageclassmap.MapIBMStorageClass(resource.Name)
			if err2 == nil {
				err = nil
			}
		}
	}

	if err != nil {
		if errors.Is(err, storageclassmap.ErrUnmappedStorageClass) {
			n.recordQuarantine("storage_class", rawClass, metric.MetricID, "storage")
			slog.Warn("ibm normalize: skipping metric due to unmapped storage class", "metric_id", metric.MetricID, "storage_class", rawClass)
			return nil, nil
		}
		return nil, fmt.Errorf("ibm normalize storage metric %s: %w", metric.MetricID, err)
	}

	priceAmount, err := extractMetricPrice(metric)
	if err != nil {
		return nil, fmt.Errorf("ibm normalize storage metric %s: %w", metric.MetricID, err)
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
				n.recordQuarantine("region", region, skuID, "storage")
				slog.Warn("ibm normalize: skipping storage SKU due to unmapped region", "sku", skuID, "region", region)
				continue
			}
			return nil, fmt.Errorf("ibm normalize storage sku %s region %s: %w", skuID, region, err)
		}

		dedupKey := fmt.Sprintf("%s:%s", skuID, region)
		if n.seen[dedupKey] {
			continue
		}
		n.seen[dedupKey] = true

		observations = append(observations, domain.PriceObservation{
			Provider:        "ibm",
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

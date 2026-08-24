package digitalocean

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/shopspring/decimal"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/regionmap"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/storageclassmap"
)

// normalizeStorageProduct converts a DigitalOcean storage product into normalized PriceObservation records.
func (n *computeNormalizer) normalizeStorageProduct(prod Product) ([]domain.PriceObservation, error) {
	rawClass := prod.Slug
	if rawClass == "" {
		rawClass = prod.Type
	}
	if rawClass == "" {
		rawClass = prod.Name
	}

	storageClass, err := storageclassmap.MapDigitalOceanStorageClass(rawClass)
	if err != nil {
		if errors.Is(err, storageclassmap.ErrUnmappedStorageClass) {
			n.recordQuarantine("storage_class", rawClass, prod.Slug, "storage")
			slog.Warn("digitalocean normalize: skipping product due to unmapped storage class", "slug", prod.Slug, "storage_class", rawClass)
			return nil, nil
		}
		return nil, fmt.Errorf("digitalocean normalize storage product %s: %w", prod.Slug, err)
	}

	priceAmount := decimal.Zero
	if prod.PricePerGB > 0 {
		priceAmount = decimal.NewFromFloat(prod.PricePerGB)
	} else if prod.PriceMonthly > 0 {
		priceAmount = decimal.NewFromFloat(prod.PriceMonthly)
	} else if prod.PriceHourly > 0 {
		priceAmount = decimal.NewFromFloat(prod.PriceHourly).Mul(decimal.NewFromInt(730))
	}

	if priceAmount.IsZero() {
		return nil, nil
	}

	regions := prod.Regions
	if len(regions) == 0 {
		n.recordQuarantine("region", "missing", prod.Slug, "storage")
		slog.Warn("digitalocean normalize: skipping storage SKU due to missing regions", "slug", prod.Slug)
		return nil, nil
	}

	cleanSlug := strings.ToUpper(strings.ReplaceAll(prod.Slug, ".", "-"))
	skuID := fmt.Sprintf("SKU-DO-STORAGE-%s", cleanSlug)
	displayName := prod.Name
	if displayName == "" {
		displayName = fmt.Sprintf("DigitalOcean Storage %s", prod.Slug)
	}

	var observations []domain.PriceObservation
	for _, region := range regions {
		regionGroup, err := regionmap.MapDigitalOceanRegion(region)
		if err != nil {
			if errors.Is(err, regionmap.ErrUnmappedRegion) {
				n.recordQuarantine("region", region, skuID, "storage")
				slog.Warn("digitalocean normalize: skipping storage SKU due to unmapped region", "sku", skuID, "region", region)
				continue
			}
			return nil, fmt.Errorf("digitalocean normalize storage sku %s region %s: %w", skuID, region, err)
		}

		dedupKey := fmt.Sprintf("%s:%s", skuID, region)
		if n.seen[dedupKey] {
			continue
		}
		n.seen[dedupKey] = true

		observations = append(observations, domain.PriceObservation{
			Provider:        "digitalocean",
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

package digitalocean

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/shopspring/decimal"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/regionmap"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/transfertypemap"
)

// normalizeNetworkProduct converts a DigitalOcean network product into normalized PriceObservation records.
func (n *computeNormalizer) normalizeNetworkProduct(prod Product) ([]domain.PriceObservation, error) {
	rawType := prod.Slug
	if rawType == "" {
		rawType = prod.Type
	}
	if rawType == "" {
		rawType = prod.Name
	}

	transferType, err := transfertypemap.MapDigitalOceanTransferType(rawType)
	if err != nil {
		if errors.Is(err, transfertypemap.ErrUnmappedTransferType) {
			n.recordQuarantine("transfer_type", rawType, prod.Slug, "network")
			slog.Warn("digitalocean normalize: skipping product due to unmapped transfer type", "slug", prod.Slug, "transfer_type", rawType)
			return nil, nil
		}
		return nil, fmt.Errorf("digitalocean normalize network product %s: %w", prod.Slug, err)
	}

	priceAmount := decimal.Zero
	if prod.PricePerGB > 0 {
		priceAmount = decimal.NewFromFloat(prod.PricePerGB)
	} else if prod.PriceMonthly > 0 {
		priceAmount = decimal.NewFromFloat(prod.PriceMonthly)
	} else if prod.PriceHourly > 0 {
		priceAmount = decimal.NewFromFloat(prod.PriceHourly)
	}

	if priceAmount.IsZero() {
		return nil, nil
	}

	regions := prod.Regions
	if len(regions) == 0 {
		n.recordQuarantine("region", "missing", prod.Slug, "network")
		slog.Warn("digitalocean normalize: skipping network SKU due to missing regions", "slug", prod.Slug)
		return nil, nil
	}

	cleanSlug := strings.ToUpper(strings.ReplaceAll(prod.Slug, ".", "-"))
	skuID := fmt.Sprintf("SKU-DO-NETWORK-%s", cleanSlug)
	displayName := prod.Name
	if displayName == "" {
		displayName = fmt.Sprintf("DigitalOcean Network %s", prod.Slug)
	}

	var observations []domain.PriceObservation
	for _, region := range regions {
		regionGroup, err := regionmap.MapDigitalOceanRegion(region)
		if err != nil {
			if errors.Is(err, regionmap.ErrUnmappedRegion) {
				n.recordQuarantine("region", region, skuID, "network")
				slog.Warn("digitalocean normalize: skipping network SKU due to unmapped region", "sku", skuID, "region", region)
				continue
			}
			return nil, fmt.Errorf("digitalocean normalize network sku %s region %s: %w", skuID, region, err)
		}

		dedupKey := fmt.Sprintf("%s:%s", skuID, region)
		if n.seen[dedupKey] {
			continue
		}
		n.seen[dedupKey] = true

		observations = append(observations, domain.PriceObservation{
			Provider:        "digitalocean",
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

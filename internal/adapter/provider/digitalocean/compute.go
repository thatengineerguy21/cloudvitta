package digitalocean

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/catalogmap"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/regionmap"
	"github.com/thatengineerguy21/CloudVitta/internal/quarantine"
)

type computeNormalizer struct {
	ctx       context.Context
	fetchedAt time.Time
	sink      quarantine.Sink
	seen      map[string]bool
}

func (n *computeNormalizer) recordQuarantine(kind, rawValue, skuID, category string) {
	if n.sink == nil {
		return
	}
	ctx := n.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	_ = n.sink.Record(ctx, quarantine.UnmappedItem{
		Provider:   "digitalocean",
		Category:   category,
		Kind:       kind,
		RawValue:   rawValue,
		SkuID:      skuID,
		ObservedAt: n.fetchedAt,
	})
}

func (n *computeNormalizer) sinkCount() int {
	if n.sink == nil {
		return 0
	}
	if counter, ok := n.sink.(interface{ Count() int }); ok {
		return counter.Count()
	}
	return 0
}

// Normalize parses a DigitalOcean Droplet sizes JSON stream and returns normalized domain observations.
// Unmapped taxonomy values are recorded to the optional quarantine sink and skipped without aborting.
func Normalize(r io.Reader, fetchedAt time.Time, sinks ...quarantine.Sink) (domain.NormalizationResult, error) {
	var sink quarantine.Sink
	if len(sinks) > 0 {
		sink = sinks[0]
	}

	var resp SizesResponse
	if err := json.NewDecoder(r).Decode(&resp); err != nil {
		return domain.NormalizationResult{}, fmt.Errorf("digitalocean normalize: decode json: %w", err)
	}

	normalizer := &computeNormalizer{
		ctx:       context.Background(),
		fetchedAt: fetchedAt,
		sink:      sink,
		seen:      make(map[string]bool),
	}

	var observations []domain.PriceObservation
	var ignoredCount int

	for _, size := range resp.Sizes {
		sinkCountBefore := normalizer.sinkCount()
		obsCountBefore := len(observations)

		obsList, err := normalizer.processSize(size)
		if err != nil {
			return domain.NormalizationResult{}, err
		}
		observations = append(observations, obsList...)

		if len(observations) == obsCountBefore && normalizer.sinkCount() == sinkCountBefore {
			slog.Debug("digitalocean normalize: ignoring out-of-scope size", "slug", size.Slug)
			ignoredCount++
		}
	}

	for _, prod := range resp.Products {
		sinkCountBefore := normalizer.sinkCount()
		obsCountBefore := len(observations)

		rawCat := prod.Type
		if rawCat == "" {
			rawCat = prod.Slug
		}
		category, err := catalogmap.MapDigitalOceanProduct(rawCat)
		if err != nil {
			if errors.Is(err, catalogmap.ErrUnmappedProduct) {
				normalizer.recordQuarantine("product", rawCat, prod.Slug, "unknown")
				slog.Warn("digitalocean normalize: skipping product due to unmapped category", "slug", prod.Slug, "type", rawCat)
				continue
			}
			return domain.NormalizationResult{}, fmt.Errorf("digitalocean normalize product %s: %w", prod.Slug, err)
		}

		switch category {
		case "storage":
			obsList, err := normalizer.normalizeStorageProduct(prod)
			if err != nil {
				return domain.NormalizationResult{}, err
			}
			observations = append(observations, obsList...)
		case "network":
			obsList, err := normalizer.normalizeNetworkProduct(prod)
			if err != nil {
				return domain.NormalizationResult{}, err
			}
			observations = append(observations, obsList...)
		}

		if len(observations) == obsCountBefore && normalizer.sinkCount() == sinkCountBefore {
			slog.Debug("digitalocean normalize: ignoring out-of-scope product", "slug", prod.Slug, "type", rawCat)
			ignoredCount++
		}
	}

	return domain.NormalizationResult{
		Observations: observations,
		IgnoredCount: ignoredCount,
	}, nil
}

func (n *computeNormalizer) processSize(size Size) ([]domain.PriceObservation, error) {
	category, err := catalogmap.MapDigitalOceanProduct("droplet")
	if err != nil {
		if errors.Is(err, catalogmap.ErrUnmappedProduct) {
			n.recordQuarantine("product", "droplet", size.Slug, "unknown")
			slog.Warn("digitalocean normalize: skipping size due to unmapped product", "slug", size.Slug)
			return nil, nil
		}
		return nil, fmt.Errorf("digitalocean normalize product droplet: %w", err)
	}

	if category != "compute" {
		return nil, nil
	}

	slug := strings.TrimSpace(size.Slug)
	if slug == "" {
		return nil, nil
	}

	vcpus := float64(size.VCPUs)
	ramGB := float64(size.Memory) / 1024.0

	if vcpus <= 0 || ramGB <= 0 {
		n.recordQuarantine("product", slug, slug, "compute")
		slog.Warn("digitalocean normalize: skipping size with zero vCPU or RAM", "slug", slug, "vcpu", vcpus, "ram", ramGB)
		return nil, nil
	}

	priceAmount := decimal.Zero
	if size.PriceHourly > 0 {
		priceAmount = decimal.NewFromFloat(size.PriceHourly)
	} else if size.PriceMonthly > 0 {
		priceAmount = decimal.NewFromFloat(size.PriceMonthly).Div(decimal.NewFromInt(730))
	}

	if priceAmount.IsZero() {
		return nil, nil
	}

	family := classifyDropletFamily(slug, size.Description)
	cleanSlug := strings.ToUpper(strings.ReplaceAll(slug, ".", "-"))
	skuID := fmt.Sprintf("SKU-DO-DROPLET-%s", cleanSlug)

	var displayName string
	if ramGB == float64(int(ramGB)) {
		displayName = fmt.Sprintf("DigitalOcean Droplet %s (%.0f vCPU, %.0f GB RAM)", slug, vcpus, ramGB)
	} else {
		displayName = fmt.Sprintf("DigitalOcean Droplet %s (%.0f vCPU, %.1f GB RAM)", slug, vcpus, ramGB)
	}

	regions := size.Regions
	if len(regions) == 0 {
		n.recordQuarantine("region", "missing", skuID, "compute")
		slog.Warn("digitalocean normalize: skipping SKU due to missing regions", "sku", skuID)
		return nil, nil
	}

	var observations []domain.PriceObservation
	for _, region := range regions {
		regionGroup, err := n.recordAndResolveRegion(region, skuID, "compute")
		if err != nil {
			return nil, fmt.Errorf("digitalocean normalize sku %s region %s: %w", skuID, region, err)
		}
		if regionGroup == "" {
			continue
		}

		dedupKey := fmt.Sprintf("%s:%s", skuID, region)
		if n.seen[dedupKey] {
			continue
		}
		n.seen[dedupKey] = true

		observations = append(observations, domain.PriceObservation{
			Provider:        "digitalocean",
			ServiceCategory: "compute",
			SkuID:           skuID,
			DisplayName:     displayName,
			Region:          region,
			RegionGroup:     regionGroup,
			Unit:            "Hrs",
			PriceAmount:     priceAmount,
			PriceCurrency:   "USD",
			PricingModel:    "OnDemand",
			Attributes: domain.ComputeAttributes{
				VCPU:   vcpus,
				RAMGB:  ramGB,
				Family: family,
			},
			FetchedAt: n.fetchedAt,
		})
	}

	return observations, nil
}

func classifyDropletFamily(slug, description string) string {
	slugLower := strings.ToLower(slug)
	descLower := strings.ToLower(description)

	switch {
	case strings.HasPrefix(slugLower, "gpu-") || strings.Contains(descLower, "gpu"):
		return "gpu"
	case strings.HasPrefix(slugLower, "c-") || strings.HasPrefix(slugLower, "c2-") || strings.Contains(descLower, "cpu-optimized") || strings.Contains(descLower, "compute-optimized") || strings.Contains(descLower, "cpu_optimized"):
		return "compute_optimized"
	case strings.HasPrefix(slugLower, "m-") || strings.HasPrefix(slugLower, "m3-") || strings.HasPrefix(slugLower, "m6-") || strings.Contains(descLower, "memory-optimized") || strings.Contains(descLower, "memory_optimized"):
		return "memory_optimized"
	case strings.HasPrefix(slugLower, "so-") || strings.HasPrefix(slugLower, "so1_5-") || strings.Contains(descLower, "storage-optimized") || strings.Contains(descLower, "storage_optimized"):
		return "storage_optimized"
	case strings.HasPrefix(slugLower, "s-") || strings.HasPrefix(slugLower, "g-") || strings.HasPrefix(slugLower, "gd-") || strings.Contains(descLower, "basic") || strings.Contains(descLower, "general purpose") || strings.Contains(descLower, "standard"):
		return "general_purpose"
	default:
		return "general_purpose"
	}
}

func (n *computeNormalizer) recordAndResolveRegion(region, skuID, category string) (string, error) {
	regionGroup, err := regionmap.MapDigitalOceanRegion(region)
	if err != nil {
		if errors.Is(err, regionmap.ErrUnmappedRegion) {
			n.recordQuarantine("region", region, skuID, category)
			slog.Warn("digitalocean normalize: skipping SKU due to unmapped region", "sku", skuID, "region", region)
			return "", nil
		}
		return "", err
	}
	return regionGroup, nil
}

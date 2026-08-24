package alibaba

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/catalogmap"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/regionmap"
	"github.com/thatengineerguy21/CloudVitta/internal/quarantine"
)

var (
	// ecsTypeRegex parses ECS instance type strings like ecs.g7.large, ecs.c6.xlarge, ecs.t6-c1m1.large, ecs.r7a.2xlarge
	ecsTypeRegex = regexp.MustCompile(`(?i)^(?:ecs\.)?([a-z0-9]+?)(?:-[a-z0-9]+)?\.[a-z0-9]+$`)
)

type computeNormalizer struct {
	fetchedAt time.Time
	sink      quarantine.Sink
	seen      map[string]bool
}

func (n *computeNormalizer) recordQuarantine(ctx context.Context, kind, rawValue, skuID, category string) {
	if n.sink != nil {
		_ = n.sink.Record(ctx, quarantine.UnmappedItem{
			Provider:   "alibaba",
			Category:   category,
			Kind:       kind,
			RawValue:   rawValue,
			SkuID:      skuID,
			ObservedAt: n.fetchedAt,
		})
	}
}

// Normalize parses an Alibaba Cloud ECS catalog or pricing JSON stream and returns normalized domain observations.
// Unmapped taxonomy values are recorded to the optional quarantine sink and skipped without aborting.
func Normalize(r io.Reader, fetchedAt time.Time, sinks ...quarantine.Sink) ([]domain.PriceObservation, error) {
	var sink quarantine.Sink
	if len(sinks) > 0 {
		sink = sinks[0]
	}

	var resp CatalogResponse
	if err := json.NewDecoder(r).Decode(&resp); err != nil {
		return nil, fmt.Errorf("alibaba normalize: decode json: %w", err)
	}

	normalizer := &computeNormalizer{
		fetchedAt: fetchedAt,
		sink:      sink,
		seen:      make(map[string]bool),
	}

	// Collect items from all possible top-level arrays
	var items []InstanceTypeItem
	if resp.InstanceTypes != nil && len(resp.InstanceTypes.List) > 0 {
		items = append(items, resp.InstanceTypes.List...)
	}
	if len(resp.Prices) > 0 {
		items = append(items, resp.Prices...)
	}
	if len(resp.Items) > 0 {
		items = append(items, resp.Items...)
	}

	var observations []domain.PriceObservation
	for _, item := range items {
		obsList, err := normalizer.processItem(item)
		if err != nil {
			return nil, err
		}
		observations = append(observations, obsList...)
	}

	return observations, nil
}

func (n *computeNormalizer) processItem(item InstanceTypeItem) ([]domain.PriceObservation, error) {
	productCode := item.ProductCode
	if productCode == "" {
		productCode = "ecs"
	}

	category, err := catalogmap.MapAlibabaProduct(productCode)
	if err != nil {
		if errors.Is(err, catalogmap.ErrUnmappedProduct) {
			n.recordQuarantine(context.Background(), "product", productCode, item.InstanceTypeID, "unknown")
			slog.Warn("alibaba normalize: skipping item due to unmapped product", "product", productCode, "instance_type", item.InstanceTypeID)
			return nil, nil
		}
		return nil, fmt.Errorf("alibaba normalize product %s: %w", productCode, err)
	}

	if category != "compute" {
		return nil, nil
	}

	instanceTypeID := strings.TrimSpace(item.InstanceTypeID)
	if instanceTypeID == "" {
		return nil, nil
	}

	family := classifyECSFamily(instanceTypeID, item.InstanceTypeFamily, item.InstanceFamily)
	if family == "" {
		n.recordQuarantine(context.Background(), "product", instanceTypeID, instanceTypeID, "compute")
		slog.Warn("alibaba normalize: skipping unrecognized instance family", "instance_type", instanceTypeID)
		return nil, nil
	}

	vcpus := item.CPUCoreCount
	ramGB := item.MemorySize
	if vcpus <= 0 || ramGB <= 0 {
		n.recordQuarantine(context.Background(), "product", instanceTypeID, instanceTypeID, "compute")
		slog.Warn("alibaba normalize: skipping instance type with zero vCPU or RAM", "instance_type", instanceTypeID, "vcpu", vcpus, "ram", ramGB)
		return nil, nil
	}

	priceAmount, currency := extractPriceAndCurrency(item)
	if priceAmount.IsZero() {
		return nil, nil
	}

	cleanType := strings.ToUpper(strings.ReplaceAll(instanceTypeID, ".", "-"))
	cleanType = strings.TrimPrefix(cleanType, "ECS-")
	skuID := fmt.Sprintf("SKU-ALI-ECS-%s", cleanType)
	displayName := fmt.Sprintf("Alibaba Cloud ECS %s (%.0f vCPU, %.0f GB RAM)", instanceTypeID, vcpus, ramGB)

	regions := item.Regions
	if len(regions) == 0 && item.RegionID != "" {
		regions = []string{item.RegionID}
	}

	if len(regions) == 0 {
		n.recordQuarantine(context.Background(), "region", "missing", skuID, "compute")
		slog.Warn("alibaba normalize: skipping SKU due to missing regions", "sku", skuID)
		return nil, nil
	}

	var observations []domain.PriceObservation
	for _, region := range regions {
		regionGroup, err := n.recordAndResolveRegion(context.Background(), region, skuID, "compute")
		if err != nil {
			return nil, fmt.Errorf("alibaba normalize sku %s region %s: %w", skuID, region, err)
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
			Provider:        "alibaba",
			ServiceCategory: "compute",
			SkuID:           skuID,
			DisplayName:     displayName,
			Region:          region,
			RegionGroup:     regionGroup,
			Unit:            "Hrs",
			PriceAmount:     priceAmount,
			PriceCurrency:   currency,
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

var ecsFamilyMap = map[string]string{
	// General Purpose
	"g5":  "general_purpose",
	"g6":  "general_purpose",
	"g6e": "general_purpose",
	"g7":  "general_purpose",
	"g7a": "general_purpose",
	"g8":  "general_purpose",
	"g8a": "general_purpose",
	"g8i": "general_purpose",
	// Compute Optimized
	"c5":  "compute_optimized",
	"c6":  "compute_optimized",
	"c6e": "compute_optimized",
	"c7":  "compute_optimized",
	"c7a": "compute_optimized",
	"c8":  "compute_optimized",
	"c8a": "compute_optimized",
	"c8i": "compute_optimized",
	// Memory Optimized
	"r5":  "memory_optimized",
	"r6":  "memory_optimized",
	"r6e": "memory_optimized",
	"r7":  "memory_optimized",
	"r7a": "memory_optimized",
	"r8":  "memory_optimized",
	"r8a": "memory_optimized",
	"r8i": "memory_optimized",
	"re6": "memory_optimized",
	// Burstable
	"t5": "burstable",
	"t6": "burstable",
	// Storage Optimized
	"i2": "storage_optimized",
	"i3": "storage_optimized",
	"i4": "storage_optimized",
	"d1": "storage_optimized",
	"d2": "storage_optimized",
	// GPU
	"gn6":  "gpu",
	"gn7":  "gpu",
	"vgn6": "gpu",
	"scc":  "gpu",
}

func classifyECSFamily(instanceTypeID, familyField1, familyField2 string) string {
	candidate := familyField1
	if candidate == "" {
		candidate = familyField2
	}
	if candidate == "" {
		matches := ecsTypeRegex.FindStringSubmatch(instanceTypeID)
		if len(matches) >= 2 {
			candidate = matches[1]
		}
	}

	clean := strings.ToLower(strings.TrimPrefix(candidate, "ecs."))
	if idx := strings.Index(clean, "-"); idx != -1 {
		clean = clean[:idx]
	}

	return ecsFamilyMap[clean]
}

func extractPriceAndCurrency(item InstanceTypeItem) (decimal.Decimal, string) {
	if item.Price != nil {
		currency := strings.ToUpper(strings.TrimSpace(item.Price.Currency))
		if currency == "" {
			currency = "USD"
		}
		if item.Price.TradePrice.IsPositive() {
			return item.Price.TradePrice, currency
		}
		if item.Price.OriginalPrice.IsPositive() {
			return item.Price.OriginalPrice, currency
		}
	}

	currency := strings.ToUpper(strings.TrimSpace(item.Currency))
	if currency == "" {
		currency = "USD"
	}

	if item.TradePrice.IsPositive() {
		return item.TradePrice, currency
	}
	if item.OriginalPrice.IsPositive() {
		return item.OriginalPrice, currency
	}

	return decimal.Zero, currency
}

func (n *computeNormalizer) recordAndResolveRegion(ctx context.Context, region, skuID, category string) (string, error) {
	regionGroup, err := regionmap.MapAlibabaRegion(region)
	if err != nil {
		if errors.Is(err, regionmap.ErrUnmappedRegion) {
			n.recordQuarantine(ctx, "region", region, skuID, category)
			slog.Warn("alibaba normalize: skipping SKU due to unmapped region", "sku", skuID, "region", region)
			return "", nil
		}
		return "", err
	}
	return regionGroup, nil
}

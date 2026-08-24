package ibm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/catalogmap"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/regionmap"
	"github.com/thatengineerguy21/CloudVitta/internal/quarantine"
)

var (
	// profileRegex matches VPC Gen 2 instance profiles like bx2-2x8, cx2-4x8, mx2-8x64, vx2-4x56, ba2-2x8, etc.
	profileRegex = regexp.MustCompile(`(?i)^(?:is\.instance\.)?([a-z]{1,2})(\d{1,2})[a-z]?-(\d+)x(\d+)$`)

	// defaultIBMRegions is the baseline list of active IBM Cloud VPC Gen 2 regions.
	defaultIBMRegions = []string{
		"us-east",
		"us-south",
		"ca-tor",
		"eu-de",
		"eu-gb",
		"eu-es",
		"jp-tok",
		"jp-osa",
		"au-syd",
		"br-sao",
	}
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
		Provider:   "ibm",
		Category:   category,
		Kind:       kind,
		RawValue:   rawValue,
		SkuID:      skuID,
		ObservedAt: n.fetchedAt,
	})
}

// Normalize parses an IBM Cloud Global Catalog API JSON stream and returns normalized domain observations.
// Unmapped taxonomy values are recorded to the optional quarantine sink and skipped without aborting.
func Normalize(r io.Reader, fetchedAt time.Time, sinks ...quarantine.Sink) ([]domain.PriceObservation, error) {
	var sink quarantine.Sink
	if len(sinks) > 0 {
		sink = sinks[0]
	}

	var resp CatalogResponse
	if err := json.NewDecoder(r).Decode(&resp); err != nil {
		return nil, fmt.Errorf("ibm normalize: decode json: %w", err)
	}

	normalizer := &computeNormalizer{
		fetchedAt: fetchedAt,
		sink:      sink,
		seen:      make(map[string]bool),
	}

	var observations []domain.PriceObservation
	for _, resource := range resp.Resources {
		obs, err := normalizer.processResource(resource, defaultIBMRegions)
		if err != nil {
			return nil, err
		}
		observations = append(observations, obs...)
	}

	return observations, nil
}

func (n *computeNormalizer) processResource(
	resource Resource,
	parentGeoTags []string,
) ([]domain.PriceObservation, error) {
	var observations []domain.PriceObservation

	geoTags := resource.GeoTags
	if len(geoTags) == 0 {
		geoTags = parentGeoTags
	}

	// Check catalog mapping if name or ID is present
	productIdentifier := resource.Name
	if productIdentifier == "" {
		productIdentifier = resource.ID
	}

	var category string
	if productIdentifier != "" {
		var err error
		category, err = catalogmap.MapIBMProduct(productIdentifier)
		if err != nil {
			if errors.Is(err, catalogmap.ErrUnmappedProduct) {
				if resource.Pricing != nil && len(resource.Pricing.Metrics) > 0 {
					n.recordQuarantine("product", productIdentifier, resource.ID, "unknown")
					slog.Warn("ibm normalize: skipping resource due to unmapped product", "id", resource.ID, "name", productIdentifier)
				}
				// Skip pricing for this unmapped resource
				return nil, nil
			}
			return nil, fmt.Errorf("ibm normalize resource %s: %w", resource.ID, err)
		}

		if category != "compute" && category != "storage" && category != "network" {
			return nil, nil
		}
	}

	// Normalize pricing metrics if present
	if resource.Pricing != nil {
		for _, metric := range resource.Pricing.Metrics {
			var metricObs []domain.PriceObservation
			var err error
			switch category {
			case "storage":
				metricObs, err = n.normalizeStorageMetric(metric, resource, geoTags)
			case "network":
				metricObs, err = n.normalizeNetworkMetric(metric, resource, geoTags)
			default:
				metricObs, err = n.normalizeMetric(metric, geoTags)
			}
			if err != nil {
				return nil, err
			}
			observations = append(observations, metricObs...)
		}
	}

	// Recursively process child resources
	for _, child := range resource.Children {
		childObs, err := n.processResource(child, geoTags)
		if err != nil {
			return nil, err
		}
		observations = append(observations, childObs...)
	}

	return observations, nil
}

func (n *computeNormalizer) normalizeMetric(
	metric PricingMetric,
	geoTags []string,
) ([]domain.PriceObservation, error) {
	metricID := strings.TrimPrefix(metric.MetricID, "is.instance.")
	metricID = strings.TrimPrefix(metricID, "instance.")

	matches := profileRegex.FindStringSubmatch(metricID)
	if len(matches) < 5 {
		n.recordQuarantine("product", metric.MetricID, metric.MetricID, "compute")
		slog.Warn("ibm normalize: skipping unrecognized VPC profile metric", "metric_id", metric.MetricID)
		return nil, nil
	}

	prefix := strings.ToLower(matches[1])
	family := classifyFamily(prefix)
	if family == "" {
		n.recordQuarantine("product", metric.MetricID, metric.MetricID, "compute")
		slog.Warn("ibm normalize: skipping unrecognized instance family", "metric_id", metric.MetricID, "prefix", prefix)
		return nil, nil
	}

	vcpus, err := strconv.ParseFloat(matches[3], 64)
	if err != nil {
		return nil, fmt.Errorf("ibm normalize: invalid vcpu %s in %s: %w", matches[3], metric.MetricID, err)
	}

	ramGB, err := strconv.ParseFloat(matches[4], 64)
	if err != nil {
		return nil, fmt.Errorf("ibm normalize: invalid ram %s in %s: %w", matches[4], metric.MetricID, err)
	}

	priceAmount, err := extractMetricPrice(metric)
	if err != nil {
		return nil, fmt.Errorf("ibm normalize metric %s: %w", metric.MetricID, err)
	}
	if priceAmount.IsZero() {
		return nil, nil
	}

	cleanProfile := strings.ToUpper(metricID)
	skuID := fmt.Sprintf("SKU-IBM-VPC-%s", cleanProfile)
	displayName := fmt.Sprintf("IBM VPC Virtual Server %s (%.0f vCPU, %.0f GB RAM)", cleanProfile, vcpus, ramGB)

	if len(geoTags) == 0 {
		n.recordQuarantine("region", "missing", skuID, "compute")
		slog.Warn("ibm normalize: skipping metric due to missing regions", "sku", skuID)
		return nil, nil
	}

	var observations []domain.PriceObservation
	for _, region := range geoTags {
		regionGroup, err := n.recordAndResolveRegion(region, skuID, "compute")
		if err != nil {
			return nil, fmt.Errorf("ibm normalize sku %s region %s: %w", skuID, region, err)
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
			Provider:        "ibm",
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

func classifyFamily(prefix string) string {
	switch prefix {
	case "b", "bx", "ba", "bz":
		return "general_purpose"
	case "c", "cx", "ca", "cz":
		return "compute_optimized"
	case "m", "mx", "ma", "mz", "v", "vx":
		return "memory_optimized"
	case "o", "ox":
		return "storage_optimized"
	case "g", "gx":
		return "gpu"
	default:
		return ""
	}
}

func extractMetricPrice(metric PricingMetric) (decimal.Decimal, error) {
	for _, amt := range metric.Amounts {
		if amt.Currency == "USD" || len(metric.Amounts) == 1 {
			for _, p := range amt.Prices {
				if p.Price.IsPositive() {
					return p.Price, nil
				}
			}
		}
	}
	return decimal.Zero, nil
}

func (n *computeNormalizer) recordAndResolveRegion(region, skuID, category string) (string, error) {
	regionGroup, err := regionmap.MapIBMRegion(region)
	if err != nil {
		if errors.Is(err, regionmap.ErrUnmappedRegion) {
			n.recordQuarantine("region", region, skuID, category)
			slog.Warn("ibm normalize: skipping SKU due to unmapped region", "sku", skuID, "region", region)
			return "", nil
		}
		return "", err
	}
	return regionGroup, nil
}

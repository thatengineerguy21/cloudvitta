package oracle

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

type flexShapeComponent struct {
	ocpuPrice decimal.Decimal
	ramPrice  decimal.Decimal
	arch      string
	family    string
	rawName   string
	regions   []string
}

type flexSizeSpec struct {
	ocpu  float64
	vcpu  float64
	ramGB float64
}

type flexFamilySpec struct {
	displayName string
	arch        string
	sizes       []flexSizeSpec
}

var flexFamilySpecs = map[string]flexFamilySpec{
	"standard_a1": {
		displayName: "Standard.A1",
		arch:        "arm64",
		sizes: []flexSizeSpec{
			{ocpu: 1, vcpu: 1, ramGB: 6},
			{ocpu: 2, vcpu: 2, ramGB: 12},
			{ocpu: 4, vcpu: 4, ramGB: 24},
			{ocpu: 8, vcpu: 8, ramGB: 48},
			{ocpu: 16, vcpu: 16, ramGB: 64},
		},
	},
	"standard_e4": {
		displayName: "Standard.E4",
		arch:        "x86_64",
		sizes: []flexSizeSpec{
			{ocpu: 1, vcpu: 2, ramGB: 8},
			{ocpu: 2, vcpu: 4, ramGB: 16},
			{ocpu: 4, vcpu: 8, ramGB: 32},
			{ocpu: 8, vcpu: 16, ramGB: 64},
			{ocpu: 16, vcpu: 32, ramGB: 128},
		},
	},
	"standard_e5": {
		displayName: "Standard.E5",
		arch:        "x86_64",
		sizes: []flexSizeSpec{
			{ocpu: 1, vcpu: 2, ramGB: 8},
			{ocpu: 2, vcpu: 4, ramGB: 16},
			{ocpu: 4, vcpu: 8, ramGB: 32},
			{ocpu: 8, vcpu: 16, ramGB: 64},
			{ocpu: 16, vcpu: 32, ramGB: 128},
		},
	},
	"standard_e3": {
		displayName: "Standard.E3",
		arch:        "x86_64",
		sizes: []flexSizeSpec{
			{ocpu: 1, vcpu: 2, ramGB: 8},
			{ocpu: 2, vcpu: 4, ramGB: 16},
			{ocpu: 4, vcpu: 8, ramGB: 32},
			{ocpu: 8, vcpu: 16, ramGB: 64},
			{ocpu: 16, vcpu: 32, ramGB: 128},
		},
	},
	"standard_x9": {
		displayName: "Standard.X9",
		arch:        "x86_64",
		sizes: []flexSizeSpec{
			{ocpu: 1, vcpu: 2, ramGB: 8},
			{ocpu: 2, vcpu: 4, ramGB: 16},
			{ocpu: 4, vcpu: 8, ramGB: 32},
			{ocpu: 8, vcpu: 16, ramGB: 64},
			{ocpu: 16, vcpu: 32, ramGB: 128},
		},
	},
	"denseio_e4": {
		displayName: "DenseIO.E4",
		arch:        "x86_64",
		sizes: []flexSizeSpec{
			{ocpu: 1, vcpu: 2, ramGB: 8},
			{ocpu: 2, vcpu: 4, ramGB: 16},
			{ocpu: 4, vcpu: 8, ramGB: 32},
			{ocpu: 8, vcpu: 16, ramGB: 64},
			{ocpu: 16, vcpu: 32, ramGB: 128},
		},
	},
	"optimized3": {
		displayName: "Optimized3",
		arch:        "x86_64",
		sizes: []flexSizeSpec{
			{ocpu: 1, vcpu: 2, ramGB: 4},
			{ocpu: 2, vcpu: 4, ramGB: 8},
			{ocpu: 4, vcpu: 8, ramGB: 16},
			{ocpu: 8, vcpu: 16, ramGB: 32},
		},
	},
}

type oracleNormalizer struct {
	ctx       context.Context
	fetchedAt time.Time
	sink      quarantine.Sink
	seen      map[string]bool
}

func (n *oracleNormalizer) recordQuarantine(kind, rawValue, skuID, category string) {
	if n.sink == nil {
		return
	}
	ctx := n.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	_ = n.sink.Record(ctx, quarantine.UnmappedItem{
		Provider:   "oracle",
		Category:   category,
		Kind:       kind,
		RawValue:   rawValue,
		SkuID:      skuID,
		ObservedAt: n.fetchedAt,
	})
}

// Normalize parses an Oracle CE Tools API JSON stream and returns normalized domain observations.
// Unmapped taxonomy values are recorded to the optional quarantine sink and skipped without aborting the page.
func Normalize(r io.Reader, fetchedAt time.Time, sinks ...quarantine.Sink) ([]domain.PriceObservation, error) {
	var sink quarantine.Sink
	if len(sinks) > 0 {
		sink = sinks[0]
	}

	norm := &oracleNormalizer{
		ctx:       context.Background(),
		fetchedAt: fetchedAt,
		sink:      sink,
		seen:      make(map[string]bool),
	}

	var resp ProductResponse
	if err := json.NewDecoder(r).Decode(&resp); err != nil {
		return nil, fmt.Errorf("oracle normalize: decode json: %w", err)
	}

	var observations []domain.PriceObservation
	flexShapes := make(map[string]*flexShapeComponent)

	for _, item := range resp.Items {
		serviceCategory := item.ServiceCategory
		if serviceCategory == "" {
			serviceCategory = item.ServiceCategoryDisplayName
		}

		category, err := catalogmap.MapOracleProduct(serviceCategory)
		if err != nil {
			if errors.Is(err, catalogmap.ErrUnmappedProduct) {
				norm.recordQuarantine("product", serviceCategory, item.PartNumber, "unknown")
				slog.Warn("oracle normalize: skipping SKU due to unmapped product", "part_number", item.PartNumber, "product", serviceCategory)
				continue
			}
			return nil, fmt.Errorf("oracle normalize part %s: %w", item.PartNumber, err)
		}

		if category == "storage" {
			storageObs, err := norm.normalizeStorageItem(item)
			if err != nil {
				return nil, err
			}
			observations = append(observations, storageObs...)
			continue
		}

		if category == "network" {
			networkObs, err := norm.normalizeNetworkItem(item)
			if err != nil {
				return nil, err
			}
			observations = append(observations, networkObs...)
			continue
		}

		if category != "compute" {
			continue
		}

		priceAmount, err := extractPrice(item)
		if err != nil {
			return nil, fmt.Errorf("oracle normalize part %s: %w", item.PartNumber, err)
		}
		if priceAmount.IsZero() {
			continue
		}

		regions := item.Regions
		if len(regions) == 0 {
			norm.recordQuarantine("region", "missing", item.PartNumber, category)
			slog.Warn("oracle normalize: skipping SKU due to missing regions", "part_number", item.PartNumber)
			continue
		}

		// Check if item is a flexible shape component or a fixed shape
		if isFlexComponent(item) {
			family, arch, isOCPU, isRAM := parseFlexInfo(item)
			if family != "" {
				comp, ok := flexShapes[family]
				if !ok {
					comp = &flexShapeComponent{
						family:  family,
						arch:    arch,
						rawName: item.DisplayName,
						regions: regions,
					}
					flexShapes[family] = comp
				}
				if isOCPU {
					comp.ocpuPrice = priceAmount
				}
				if isRAM {
					comp.ramPrice = priceAmount
				}
				if len(item.Regions) > 0 {
					comp.regions = item.Regions
				}
			} else {
				norm.recordQuarantine("product", item.DisplayName, item.PartNumber, category)
				slog.Warn("oracle normalize: skipping unrecognized flex shape component", "part_number", item.PartNumber, "name", item.DisplayName)
			}
			continue
		}

		// Fixed shape normalization
		fixedObs, err := normalizeFixedShape(item, category, priceAmount, regions, fetchedAt, sink)
		if err != nil {
			return nil, err
		}
		observations = append(observations, fixedObs...)
	}

	// Synthesize discrete compute instances from collected flexible shape components
	for _, comp := range flexShapes {
		synthObs, err := synthesizeFlexShapes(comp, fetchedAt, sink)
		if err != nil {
			return nil, err
		}
		observations = append(observations, synthObs...)
	}

	return observations, nil
}

func extractPrice(item ProductItem) (decimal.Decimal, error) {
	for _, cp := range item.Prices {
		if cp.CurrencyCode == "USD" || len(item.Prices) == 1 {
			for _, p := range cp.Prices {
				if p.Model == "PAY_AS_YOU_GO" || len(cp.Prices) == 1 {
					return decimal.NewFromFloat(p.Value), nil
				}
			}
		}
	}
	return decimal.Zero, nil
}

func isFlexComponent(item ProductItem) bool {
	text := strings.ToLower(item.DisplayName + " " + item.Description + " " + item.MetricName)
	return strings.Contains(text, "flex") ||
		(strings.Contains(text, "ocpu") && (strings.Contains(text, "e4") || strings.Contains(text, "e5") || strings.Contains(text, "e3") || strings.Contains(text, "a1") || strings.Contains(text, "x9") || strings.Contains(text, "optimized3") || strings.Contains(text, "denseio"))) ||
		(strings.Contains(text, "memory") && (strings.Contains(text, "e4") || strings.Contains(text, "e5") || strings.Contains(text, "e3") || strings.Contains(text, "a1") || strings.Contains(text, "x9") || strings.Contains(text, "optimized3") || strings.Contains(text, "denseio")))
}

func parseFlexInfo(item ProductItem) (family string, arch string, isOCPU bool, isRAM bool) {
	text := strings.ToLower(item.DisplayName + " " + item.Description + " " + item.MetricName)

	switch {
	case strings.Contains(text, "a1"):
		family = "standard_a1"
	case strings.Contains(text, "e5"):
		family = "standard_e5"
	case strings.Contains(text, "e4") && strings.Contains(text, "denseio"):
		family = "denseio_e4"
	case strings.Contains(text, "e4"):
		family = "standard_e4"
	case strings.Contains(text, "e3"):
		family = "standard_e3"
	case strings.Contains(text, "x9"):
		family = "standard_x9"
	case strings.Contains(text, "optimized3") || strings.Contains(text, "optimized 3"):
		family = "optimized3"
	default:
		return "", "", false, false
	}

	spec, ok := flexFamilySpecs[family]
	if !ok {
		return "", "", false, false
	}

	arch = spec.arch

	if strings.Contains(text, "ocpu") {
		isOCPU = true
	}
	if strings.Contains(text, "memory") || strings.Contains(text, "gigabyte") || strings.Contains(text, "ram") {
		isRAM = true
	}

	return family, arch, isOCPU, isRAM
}

func synthesizeFlexShapes(comp *flexShapeComponent, fetchedAt time.Time, sink quarantine.Sink) ([]domain.PriceObservation, error) {
	if comp.ocpuPrice.IsZero() || comp.ramPrice.IsZero() {
		// Both OCPU and RAM pricing components are required to calculate total flex shape cost
		return nil, nil
	}

	spec, ok := flexFamilySpecs[comp.family]
	if !ok {
		return nil, nil
	}

	var observations []domain.PriceObservation

	for _, size := range spec.sizes {
		ocpuCost := comp.ocpuPrice.Mul(decimal.NewFromFloat(size.ocpu))
		ramCost := comp.ramPrice.Mul(decimal.NewFromFloat(size.ramGB))
		totalHourlyCost := ocpuCost.Add(ramCost)

		skuFamily := strings.ToUpper(strings.ReplaceAll(comp.family, "_", "-"))
		skuID := fmt.Sprintf("SKU-OCI-VM-%s-FLEX-%.0fVCPU-%.0fGB", skuFamily, size.vcpu, size.ramGB)
		displayName := fmt.Sprintf("VM.%s.Flex (%.0f vCPU, %.0f GB RAM)", spec.displayName, size.vcpu, size.ramGB)

		for _, region := range comp.regions {
			regionGroup, err := recordAndResolveRegion(context.Background(), region, skuID, "compute", fetchedAt, sink)
			if err != nil {
				return nil, fmt.Errorf("oracle normalize flex sku %s: %w", skuID, err)
			}
			if regionGroup == "" {
				continue
			}

			observations = append(observations, domain.PriceObservation{
				Provider:        "oracle",
				ServiceCategory: "compute",
				SkuID:           skuID,
				DisplayName:     displayName,
				Region:          region,
				RegionGroup:     regionGroup,
				Unit:            "Hrs",
				PriceAmount:     totalHourlyCost,
				PriceCurrency:   "USD",
				PricingModel:    "OnDemand",
				Attributes: domain.ComputeAttributes{
					VCPU:   size.vcpu,
					RAMGB:  size.ramGB,
					Family: comp.family,
				},
				FetchedAt: fetchedAt,
			})
		}
	}

	return observations, nil
}

var fixedShapeSpecs = map[string]domain.ComputeAttributes{
	"standard2.1":    {VCPU: 2, RAMGB: 15, Family: "standard2"},
	"standard2.2":    {VCPU: 4, RAMGB: 30, Family: "standard2"},
	"standard2.4":    {VCPU: 8, RAMGB: 60, Family: "standard2"},
	"standard2.8":    {VCPU: 16, RAMGB: 120, Family: "standard2"},
	"standard2.16":   {VCPU: 32, RAMGB: 240, Family: "standard2"},
	"standard2.24":   {VCPU: 48, RAMGB: 320, Family: "standard2"},
	"standard.e2.1":  {VCPU: 2, RAMGB: 8, Family: "standard_e2"},
	"standard.e2.2":  {VCPU: 4, RAMGB: 16, Family: "standard_e2"},
	"standard.e2.4":  {VCPU: 8, RAMGB: 32, Family: "standard_e2"},
	"standard.e2.8":  {VCPU: 16, RAMGB: 64, Family: "standard_e2"},
	"standard.b1.ms": {VCPU: 1, RAMGB: 2, Family: "burstable"},
	"standard.b1.s":  {VCPU: 1, RAMGB: 4, Family: "burstable"},
	"standard.b1.m":  {VCPU: 2, RAMGB: 8, Family: "burstable"},
	"standard.b1.l":  {VCPU: 4, RAMGB: 16, Family: "burstable"},
}

var fixedShapeRegex = regexp.MustCompile(`(?i)(?:vm\.)?(standard2\.\d+|standard\.e2\.\d+|standard\.b1\.(?:ms|s|m|l))`)

func normalizeFixedShape(item ProductItem, category string, price decimal.Decimal, regions []string, fetchedAt time.Time, sink quarantine.Sink) ([]domain.PriceObservation, error) {
	text := item.DisplayName + " " + item.Description
	matches := fixedShapeRegex.FindStringSubmatch(text)
	if len(matches) < 2 {
		return nil, nil
	}

	shapeKey := strings.ToLower(matches[1])
	attrs, ok := fixedShapeSpecs[shapeKey]
	if !ok {
		return nil, nil
	}

	cleanShapeName := strings.ToUpper(strings.ReplaceAll(shapeKey, ".", "-"))
	skuID := fmt.Sprintf("SKU-OCI-VM-%s", cleanShapeName)
	if item.PartNumber != "" {
		skuID = fmt.Sprintf("SKU-OCI-%s-%s", item.PartNumber, cleanShapeName)
	}

	var observations []domain.PriceObservation
	for _, region := range regions {
		regionGroup, err := recordAndResolveRegion(context.Background(), region, skuID, category, fetchedAt, sink)
		if err != nil {
			return nil, fmt.Errorf("oracle normalize fixed sku %s: %w", skuID, err)
		}
		if regionGroup == "" {
			continue
		}

		observations = append(observations, domain.PriceObservation{
			Provider:        "oracle",
			ServiceCategory: category,
			SkuID:           skuID,
			DisplayName:     fmt.Sprintf("VM.%s", strings.Title(shapeKey)),
			Region:          region,
			RegionGroup:     regionGroup,
			Unit:            "Hrs",
			PriceAmount:     price,
			PriceCurrency:   "USD",
			PricingModel:    "OnDemand",
			Attributes:      attrs,
			FetchedAt:       fetchedAt,
		})
	}

	return observations, nil
}

func recordAndResolveRegion(ctx context.Context, region, skuID, category string, fetchedAt time.Time, sink quarantine.Sink) (string, error) {
	regionGroup, err := regionmap.MapOracleRegion(region)
	if err != nil {
		if errors.Is(err, regionmap.ErrUnmappedRegion) {
			if sink != nil {
				_ = sink.Record(ctx, quarantine.UnmappedItem{
					Provider:   "oracle",
					Category:   category,
					Kind:       "region",
					RawValue:   region,
					SkuID:      skuID,
					ObservedAt: fetchedAt,
				})
			}
			slog.Warn("oracle normalize: skipping SKU due to unmapped region", "sku", skuID, "region", region)
			return "", nil
		}
		return "", err
	}
	return regionGroup, nil
}

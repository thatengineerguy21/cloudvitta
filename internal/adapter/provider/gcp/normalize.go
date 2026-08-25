package gcp

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
	"github.com/thatengineerguy21/CloudVitta/internal/adapter/provider"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/catalogmap"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/databaseenginemap"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/nosqldatamodelmap"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/regionmap"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/storageclassmap"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/transfertypemap"
	"github.com/thatengineerguy21/CloudVitta/internal/quarantine"
)

type gcpUnitPrice struct {
	CurrencyCode string `json:"currencyCode"`
	Units        string `json:"units"`
	Nanos        int    `json:"nanos"`
}

type gcpTieredRate struct {
	StartUsageAmount float64      `json:"startUsageAmount"`
	UnitPrice        gcpUnitPrice `json:"unitPrice"`
}

type gcpPricingExpression struct {
	UsageUnit                string          `json:"usageUnit"`
	UsageUnitDescription     string          `json:"usageUnitDescription"`
	BaseUnit                 string          `json:"baseUnit"`
	BaseUnitDescription      string          `json:"baseUnitDescription"`
	BaseUnitConversionFactor float64         `json:"baseUnitConversionFactor"`
	DisplayQuantity          float64         `json:"displayQuantity"`
	TieredRates              []gcpTieredRate `json:"tieredRates"`
}

type gcpPricingInfo struct {
	Summary                string               `json:"summary"`
	PricingExpression      gcpPricingExpression `json:"pricingExpression"`
	CurrencyConversionRate float64              `json:"currencyConversionRate"`
	EffectiveTime          string               `json:"effectiveTime"`
}

type gcpCategory struct {
	ServiceDisplayName string `json:"serviceDisplayName"`
	ResourceFamily     string `json:"resourceFamily"`
	ResourceGroup      string `json:"resourceGroup"`
	UsageType          string `json:"usageType"`
}

type gcpSKU struct {
	Name                string           `json:"name"`
	SkuID               string           `json:"skuId"`
	Description         string           `json:"description"`
	Category            gcpCategory      `json:"category"`
	ServiceRegions      []string         `json:"serviceRegions"`
	PricingInfo         []gcpPricingInfo `json:"pricingInfo"`
	ServiceProviderName string           `json:"serviceProviderName"`
}

var knownGCPVMSpecs = map[string]domain.ComputeAttributes{
	"n2-standard-2":  {VCPU: 2, RAMGB: 8, Family: "n2"},
	"n2-standard-4":  {VCPU: 4, RAMGB: 16, Family: "n2"},
	"n2-standard-8":  {VCPU: 8, RAMGB: 32, Family: "n2"},
	"n2-standard-16": {VCPU: 16, RAMGB: 64, Family: "n2"},
	"n2-standard-32": {VCPU: 32, RAMGB: 128, Family: "n2"},
	"n2-standard-48": {VCPU: 48, RAMGB: 192, Family: "n2"},
	"n2-standard-64": {VCPU: 64, RAMGB: 256, Family: "n2"},
	"n2-standard-80": {VCPU: 80, RAMGB: 320, Family: "n2"},

	"n2d-standard-2":  {VCPU: 2, RAMGB: 8, Family: "n2d"},
	"n2d-standard-4":  {VCPU: 4, RAMGB: 16, Family: "n2d"},
	"n2d-standard-8":  {VCPU: 8, RAMGB: 32, Family: "n2d"},
	"n2d-standard-16": {VCPU: 16, RAMGB: 64, Family: "n2d"},
	"n2d-standard-32": {VCPU: 32, RAMGB: 128, Family: "n2d"},
	"n2d-standard-48": {VCPU: 48, RAMGB: 192, Family: "n2d"},
	"n2d-standard-64": {VCPU: 64, RAMGB: 256, Family: "n2d"},
	"n2d-standard-80": {VCPU: 80, RAMGB: 320, Family: "n2d"},

	"n1-standard-1":  {VCPU: 1, RAMGB: 3.75, Family: "n1"},
	"n1-standard-2":  {VCPU: 2, RAMGB: 7.5, Family: "n1"},
	"n1-standard-4":  {VCPU: 4, RAMGB: 15, Family: "n1"},
	"n1-standard-8":  {VCPU: 8, RAMGB: 30, Family: "n1"},
	"n1-standard-16": {VCPU: 16, RAMGB: 60, Family: "n1"},
	"n1-standard-32": {VCPU: 32, RAMGB: 120, Family: "n1"},
	"n1-standard-64": {VCPU: 64, RAMGB: 240, Family: "n1"},

	"e2-standard-2":  {VCPU: 2, RAMGB: 8, Family: "e2"},
	"e2-standard-4":  {VCPU: 4, RAMGB: 16, Family: "e2"},
	"e2-standard-8":  {VCPU: 8, RAMGB: 32, Family: "e2"},
	"e2-standard-16": {VCPU: 16, RAMGB: 64, Family: "e2"},

	"c2-standard-4":  {VCPU: 4, RAMGB: 16, Family: "c2"},
	"c2-standard-8":  {VCPU: 8, RAMGB: 32, Family: "c2"},
	"c2-standard-16": {VCPU: 16, RAMGB: 64, Family: "c2"},

	"c2d-standard-2":  {VCPU: 2, RAMGB: 8, Family: "c2d"},
	"c2d-standard-4":  {VCPU: 4, RAMGB: 16, Family: "c2d"},
	"c2d-standard-8":  {VCPU: 8, RAMGB: 32, Family: "c2d"},
	"c2d-standard-16": {VCPU: 16, RAMGB: 64, Family: "c2d"},

	"c3-standard-4":  {VCPU: 4, RAMGB: 16, Family: "c3"},
	"c3-standard-8":  {VCPU: 8, RAMGB: 32, Family: "c3"},
	"c3-standard-22": {VCPU: 22, RAMGB: 88, Family: "c3"},
	"c3-standard-44": {VCPU: 44, RAMGB: 176, Family: "c3"},

	"t2d-standard-1": {VCPU: 1, RAMGB: 4, Family: "t2d"},
	"t2d-standard-2": {VCPU: 2, RAMGB: 8, Family: "t2d"},
	"t2d-standard-4": {VCPU: 4, RAMGB: 16, Family: "t2d"},
	"t2d-standard-8": {VCPU: 8, RAMGB: 32, Family: "t2d"},

	"t2a-standard-1": {VCPU: 1, RAMGB: 4, Family: "t2a"},
	"t2a-standard-2": {VCPU: 2, RAMGB: 8, Family: "t2a"},
	"t2a-standard-4": {VCPU: 4, RAMGB: 16, Family: "t2a"},
	"t2a-standard-8": {VCPU: 8, RAMGB: 32, Family: "t2a"},
}

// Normalize parses a GCP Cloud Billing Catalog API JSON stream and returns normalized domain observations and the next page token.
type gcpComponentSpec struct {
	corePrice decimal.Decimal
	ramPrice  decimal.Decimal
	unit      string
	currency  string
	regions   []string
}

// Normalize parses a GCP Cloud Billing Catalog API JSON stream and returns normalized domain observations and the next page token.
// Unmapped taxonomy values are recorded to the optional quarantine sink and skipped without aborting the page.
func Normalize(r io.Reader, fetchedAt time.Time, sinks ...quarantine.Sink) (domain.NormalizationResult, string, error) {
	var sink quarantine.Sink
	if len(sinks) > 0 {
		sink = sinks[0]
	}

	dec := json.NewDecoder(r)

	var observations []domain.PriceObservation
	var nextPageToken string
	var ignoredCount int
	components := make(map[string]*gcpComponentSpec)

	// Advance to the first token
	t, err := dec.Token()
	if err != nil {
		if err == io.EOF {
			return domain.NormalizationResult{}, "", nil
		}
		return domain.NormalizationResult{}, "", fmt.Errorf("gcp normalize: %w", err)
	}

	if delim, ok := t.(json.Delim); !ok || delim != '{' {
		return domain.NormalizationResult{}, "", fmt.Errorf("gcp normalize: expected '{' at start of response")
	}

	for dec.More() {
		t, err := dec.Token()
		if err != nil {
			return domain.NormalizationResult{}, "", fmt.Errorf("gcp normalize: read key: %w", err)
		}
		key, ok := t.(string)
		if !ok {
			continue
		}

		switch key {
		case "skus":
			t, err = dec.Token()
			if err != nil {
				return domain.NormalizationResult{}, "", fmt.Errorf("gcp normalize: read 'skus' value: %w", err)
			}
			if delim, ok := t.(json.Delim); !ok || delim != '[' {
				return domain.NormalizationResult{}, "", fmt.Errorf("gcp normalize: expected '[' after 'skus'")
			}

			for dec.More() {
				var sku gcpSKU
				if err := dec.Decode(&sku); err != nil {
					return domain.NormalizationResult{}, "", fmt.Errorf("gcp normalize: decode sku: %w", err)
				}

				sinkCountBefore := 0
				if sink != nil {
					if counter, ok := sink.(interface{ Count() int }); ok {
						sinkCountBefore = counter.Count()
					}
				}
				obsCountBefore := len(observations)

				// Network SKUs published under Compute Engine service display name must route to network
				if isNetworkProduct(sku) {
					skuObs, nErr := normalizeNetworkSKU(sku, "network", fetchedAt, sink)
					if nErr != nil {
						return domain.NormalizationResult{}, "", nErr
					}
					observations = append(observations, skuObs...)
					continue
				}

				// Filter 1: Check service category mapping (fails loudly if unmapped)
				serviceName := sku.Category.ServiceDisplayName
				if serviceName == "" {
					serviceName = sku.Category.ResourceFamily
				}
				category, err := catalogmap.MapGCPProduct(serviceName)
				if err != nil {
					if errors.Is(err, catalogmap.ErrUnmappedProduct) {
						if sink != nil {
							_ = sink.Record(context.Background(), quarantine.UnmappedItem{
								Provider:   "gcp",
								Category:   "unknown",
								Kind:       "product",
								RawValue:   serviceName,
								SkuID:      sku.SkuID,
								ObservedAt: fetchedAt,
							})
						}
						slog.Warn("gcp normalize: skipping SKU due to unmapped product", "sku", sku.SkuID, "product", serviceName)
						continue
					}
					return domain.NormalizationResult{}, "", fmt.Errorf("gcp normalize sku %s: %w", sku.SkuID, err)
				}

				var skuObs []domain.PriceObservation
				switch category {
				case "compute":
					if isComputeInstance(sku) {
						skuObs, err = normalizeComputeSKU(sku, category, fetchedAt, sink)
					}
					if isComputeComponent(sku) {
						collectComponentPricing(sku, components)
					}
				case "storage":
					if isStorageProduct(sku) {
						skuObs, err = normalizeStorageSKU(sku, category, fetchedAt, sink)
					}
				case "network":
					skuObs, err = normalizeNetworkSKU(sku, category, fetchedAt, sink)
				case "database_rdbms":
					if isDatabaseProduct(sku) {
						skuObs, err = normalizeDatabaseSKU(sku, category, fetchedAt, sink)
					}
				case "database_nosql":
					skuObs, err = normalizeDatabaseNoSQLSKU(sku, category, fetchedAt, sink)
				case "kubernetes":
					if isKubernetesProduct(sku) {
						skuObs, err = normalizeKubernetesSKU(sku, category, fetchedAt, sink)
					}
				case "serverless":
					skuObs, err = normalizeServerlessSKU(sku, category, fetchedAt, sink)
				}
				if err != nil {
					return domain.NormalizationResult{}, "", err
				}
				observations = append(observations, skuObs...)

				if len(observations) == obsCountBefore && !isComputeComponent(sku) {
					sinkCountAfter := 0
					if sink != nil {
						if counter, ok := sink.(interface{ Count() int }); ok {
							sinkCountAfter = counter.Count()
						}
					}
					if sinkCountAfter == sinkCountBefore {
						slog.Debug("gcp normalize: ignoring out-of-scope SKU", "sku", sku.SkuID, "desc", sku.Description)
						ignoredCount++
					}
				}
			}
			// Consume ']'
			if _, err := dec.Token(); err != nil {
				return domain.NormalizationResult{}, "", err
			}
		case "nextPageToken":
			if err := dec.Decode(&nextPageToken); err != nil {
				return domain.NormalizationResult{}, "", fmt.Errorf("gcp normalize: decode nextPageToken: %w", err)
			}
		default:
			// Discard other top-level keys
			if err := provider.SkipJSONValue(dec); err != nil {
				return domain.NormalizationResult{}, "", fmt.Errorf("gcp normalize: skip key %s: %w", key, err)
			}
		}
	}

	// Synthesize machine type pricing from collected compute component SKUs
	if len(components) > 0 {
		composedObs, err := composeMachineTypePricing(components, fetchedAt, sink)
		if err != nil {
			return domain.NormalizationResult{}, "", err
		}
		observations = append(observations, composedObs...)
	}

	return domain.NormalizationResult{
		Observations: observations,
		IgnoredCount: ignoredCount,
	}, nextPageToken, nil
}

func extractUnitPrice(unitPrice gcpUnitPrice) (decimal.Decimal, error) {
	var unitsDec decimal.Decimal
	if unitPrice.Units != "" {
		var err error
		unitsDec, err = decimal.NewFromString(unitPrice.Units)
		if err != nil {
			return decimal.Zero, fmt.Errorf("invalid units %q: %w", unitPrice.Units, err)
		}
	} else {
		unitsDec = decimal.Zero
	}

	nanosDec := decimal.NewFromInt(int64(unitPrice.Nanos)).Div(decimal.NewFromInt(1_000_000_000))
	return unitsDec.Add(nanosDec), nil
}

func normalizeComputeSKU(sku gcpSKU, category string, fetchedAt time.Time, sink quarantine.Sink) ([]domain.PriceObservation, error) {
	if !isComputeInstance(sku) {
		return nil, nil
	}
	if len(sku.PricingInfo) == 0 || len(sku.PricingInfo[0].PricingExpression.TieredRates) == 0 {
		return nil, nil
	}

	unitPrice := sku.PricingInfo[0].PricingExpression.TieredRates[0].UnitPrice
	priceAmount, err := extractUnitPrice(unitPrice)
	if err != nil {
		return nil, fmt.Errorf("gcp normalize sku %s: %w", sku.SkuID, err)
	}
	if priceAmount.IsZero() {
		return nil, nil
	}

	attrs, ok := parseGCPAttributes(sku.Description, sku.Name)
	if !ok {
		slog.Warn("gcp normalize: skipping SKU due to unmapped machine type", "provider", "gcp", "sku", sku.SkuID, "description", sku.Description, "name", sku.Name)
		return nil, nil
	}

	unit := sku.PricingInfo[0].PricingExpression.UsageUnit
	if unit == "h" || unit == "hour" {
		unit = "Hrs"
	}
	currency := unitPrice.CurrencyCode
	if currency == "" {
		currency = "USD"
	}
	regions := sku.ServiceRegions
	if len(regions) == 0 {
		regions = []string{"global"}
	}

	var results []domain.PriceObservation
	for _, region := range regions {
		regionGroup, err := regionmap.MapGCPRegion(region)
		if err != nil {
			if errors.Is(err, regionmap.ErrUnmappedRegion) {
				if sink != nil {
					_ = sink.Record(context.Background(), quarantine.UnmappedItem{
						Provider:   "gcp",
						Category:   category,
						Kind:       "region",
						RawValue:   region,
						SkuID:      sku.SkuID,
						ObservedAt: fetchedAt,
					})
				}
				slog.Warn("gcp normalize: skipping SKU due to unmapped region", "sku", sku.SkuID, "region", region)
				continue
			}
			return nil, fmt.Errorf("gcp normalize sku %s: %w", sku.SkuID, err)
		}

		results = append(results, domain.PriceObservation{
			Provider:        "gcp",
			ServiceCategory: category,
			SkuID:           sku.SkuID,
			DisplayName:     sku.Description,
			Region:          region,
			RegionGroup:     regionGroup,
			Unit:            unit,
			PriceAmount:     priceAmount,
			PriceCurrency:   currency,
			PricingModel:    "OnDemand",
			Attributes:      attrs,
			FetchedAt:       fetchedAt,
		})
	}
	return results, nil
}

func normalizeStorageSKU(sku gcpSKU, category string, fetchedAt time.Time, sink quarantine.Sink) ([]domain.PriceObservation, error) {
	if !isStorageProduct(sku) {
		return nil, nil
	}
	if len(sku.PricingInfo) == 0 || len(sku.PricingInfo[0].PricingExpression.TieredRates) == 0 {
		return nil, nil
	}

	unitPrice := sku.PricingInfo[0].PricingExpression.TieredRates[0].UnitPrice
	priceAmount, err := extractUnitPrice(unitPrice)
	if err != nil {
		return nil, fmt.Errorf("gcp normalize sku %s: %w", sku.SkuID, err)
	}
	if priceAmount.IsZero() {
		return nil, nil
	}

	storageClass, err := parseGCPStorageClass(sku.Category.ResourceGroup, sku.Description, sku.Name)
	if err != nil {
		if errors.Is(err, storageclassmap.ErrUnmappedStorageClass) {
			if sink != nil {
				_ = sink.Record(context.Background(), quarantine.UnmappedItem{
					Provider:   "gcp",
					Category:   category,
					Kind:       "storage_class",
					RawValue:   sku.Category.ResourceGroup,
					SkuID:      sku.SkuID,
					ObservedAt: fetchedAt,
				})
			}
			slog.Warn("gcp normalize: skipping SKU due to unmapped storage class", "sku", sku.SkuID, "resource_group", sku.Category.ResourceGroup)
			return nil, nil
		}
		return nil, fmt.Errorf("gcp normalize sku %s: %w", sku.SkuID, err)
	}

	unit := "GB-Mo"
	currency := unitPrice.CurrencyCode
	if currency == "" {
		currency = "USD"
	}
	regions := sku.ServiceRegions
	if len(regions) == 0 {
		regions = []string{"global"}
	}

	var results []domain.PriceObservation
	for _, region := range regions {
		regionGroup, err := regionmap.MapGCPRegion(region)
		if err != nil {
			if errors.Is(err, regionmap.ErrUnmappedRegion) {
				if sink != nil {
					_ = sink.Record(context.Background(), quarantine.UnmappedItem{
						Provider:   "gcp",
						Category:   category,
						Kind:       "region",
						RawValue:   region,
						SkuID:      sku.SkuID,
						ObservedAt: fetchedAt,
					})
				}
				slog.Warn("gcp normalize: skipping SKU due to unmapped region", "sku", sku.SkuID, "region", region)
				continue
			}
			return nil, fmt.Errorf("gcp normalize sku %s: %w", sku.SkuID, err)
		}

		results = append(results, domain.PriceObservation{
			Provider:        "gcp",
			ServiceCategory: category,
			SkuID:           sku.SkuID,
			DisplayName:     sku.Description,
			Region:          region,
			RegionGroup:     regionGroup,
			Unit:            unit,
			PriceAmount:     priceAmount,
			PriceCurrency:   currency,
			PricingModel:    "OnDemand",
			StorageAttributes: domain.StorageAttributes{
				SizeGB:       1,
				StorageClass: storageClass,
			},
			FetchedAt: fetchedAt,
		})
	}
	return results, nil
}

func normalizeNetworkSKU(sku gcpSKU, category string, fetchedAt time.Time, sink quarantine.Sink) ([]domain.PriceObservation, error) {
	if !isNetworkProduct(sku) {
		return nil, nil
	}
	if len(sku.PricingInfo) == 0 || len(sku.PricingInfo[0].PricingExpression.TieredRates) == 0 {
		return nil, nil
	}

	rates := sku.PricingInfo[0].PricingExpression.TieredRates
	if len(rates) > 1 {
		slog.Debug("skipping GCP SKU due to tiered pricing", "provider", "gcp", "sku", sku.SkuID, "tiered_rate_count", len(rates), "reason", "tiered_pricing_not_supported_in_v1")
		return nil, nil
	}
	rate := rates[0]
	if rate.StartUsageAmount > 0 {
		slog.Debug("skipping GCP SKU due to non-zero start usage tiered pricing", "provider", "gcp", "sku", sku.SkuID, "reason", "tiered_pricing_not_supported_in_v1")
		return nil, nil
	}

	unitPrice := rate.UnitPrice
	priceAmount, err := extractUnitPrice(unitPrice)
	if err != nil {
		return nil, fmt.Errorf("gcp normalize sku %s: %w", sku.SkuID, err)
	}
	if priceAmount.IsZero() {
		return nil, nil
	}

	unit := "GB"
	currency := unitPrice.CurrencyCode
	if currency == "" {
		currency = "USD"
	}
	regions := sku.ServiceRegions
	if len(regions) == 0 {
		regions = []string{"us-east1"}
	}

	transferType, err := parseGCPTransferType(sku.Description, sku.Category.ResourceGroup)
	if err != nil {
		if errors.Is(err, transfertypemap.ErrUnmappedTransferType) {
			if sink != nil {
				_ = sink.Record(context.Background(), quarantine.UnmappedItem{
					Provider:   "gcp",
					Category:   category,
					Kind:       "transfer_type",
					RawValue:   sku.Description,
					SkuID:      sku.SkuID,
					ObservedAt: fetchedAt,
				})
			}
			slog.Warn("gcp normalize: skipping SKU due to unmapped transfer type", "sku", sku.SkuID, "description", sku.Description)
			return nil, nil
		}
		return nil, fmt.Errorf("gcp normalize sku %s transfer type: %w", sku.SkuID, err)
	}

	var results []domain.PriceObservation
	for _, region := range regions {
		regionGroup, err := regionmap.MapGCPRegion(region)
		if err != nil {
			if errors.Is(err, regionmap.ErrUnmappedRegion) {
				if sink != nil {
					_ = sink.Record(context.Background(), quarantine.UnmappedItem{
						Provider:   "gcp",
						Category:   category,
						Kind:       "region",
						RawValue:   region,
						SkuID:      sku.SkuID,
						ObservedAt: fetchedAt,
					})
				}
				slog.Warn("gcp normalize: skipping SKU due to unmapped region", "sku", sku.SkuID, "region", region)
				continue
			}
			return nil, fmt.Errorf("gcp normalize sku %s: %w", sku.SkuID, err)
		}

		results = append(results, domain.PriceObservation{
			Provider:        "gcp",
			ServiceCategory: category,
			SkuID:           sku.SkuID,
			DisplayName:     sku.Description,
			Region:          region,
			RegionGroup:     regionGroup,
			Unit:            unit,
			PriceAmount:     priceAmount,
			PriceCurrency:   currency,
			PricingModel:    "OnDemand",
			NetworkAttributes: domain.NetworkAttributes{
				EgressGB:     1,
				TransferType: transferType,
			},
			FetchedAt: fetchedAt,
		})
	}
	return results, nil
}

func isNetworkProduct(sku gcpSKU) bool {
	if sku.Category.UsageType != "OnDemand" {
		return false
	}
	if sku.Category.ResourceFamily == "Network" || strings.Contains(sku.Description, "Network") || strings.Contains(sku.Description, "Egress") || strings.Contains(sku.Description, "Data Transfer") || strings.Contains(sku.Category.ResourceGroup, "Interconnect") || strings.Contains(sku.Category.ResourceGroup, "Egress") {
		return true
	}
	return false
}

func isDatabaseProduct(sku gcpSKU) bool {
	if sku.Category.UsageType != "OnDemand" {
		return false
	}
	desc := sku.Description
	family := sku.Category.ResourceFamily
	group := sku.Category.ResourceGroup

	// Exclude network egress/data transfer line items
	if family == "Network" || strings.Contains(desc, "Network") || strings.Contains(desc, "Egress") ||
		strings.Contains(desc, "Data Transfer") || strings.Contains(desc, "Internet") ||
		strings.Contains(group, "Egress") || strings.Contains(group, "Interconnect") {
		return false
	}
	// Exclude commitments, discounts, licenses, and non-database services
	if strings.Contains(desc, "Commitment") || strings.Contains(desc, "Discount") || strings.Contains(desc, "License") {
		return false
	}
	// Must be Cloud SQL, AlloyDB, or database instance/storage resource
	if strings.Contains(desc, "Cloud SQL") || strings.Contains(desc, "AlloyDB") ||
		strings.Contains(family, "ApplicationServices") || strings.Contains(group, "SQLServer") ||
		strings.Contains(group, "PostgreSQL") || strings.Contains(group, "MySQL") ||
		strings.Contains(group, "PD") || strings.Contains(desc, "Storage") ||
		strings.Contains(desc, "Instance") || strings.Contains(desc, "Core") || strings.Contains(desc, "RAM") {
		return true
	}
	return false
}

func isKubernetesProduct(sku gcpSKU) bool {
	desc := strings.ToLower(sku.Description)
	return strings.Contains(desc, "kubernetes") || strings.Contains(desc, "gke") ||
		strings.Contains(desc, "cluster management") || strings.Contains(strings.ToLower(sku.Category.ServiceDisplayName), "kubernetes")
}

func isComputeComponent(sku gcpSKU) bool {
	if sku.Category.UsageType != "OnDemand" {
		return false
	}
	desc := sku.Description
	if strings.Contains(desc, "Windows") || strings.Contains(desc, "SQL Server") || strings.Contains(desc, "RHEL") || strings.Contains(desc, "SLES") ||
		strings.Contains(desc, "Spot") || strings.Contains(desc, "Preemptible") || strings.Contains(desc, "Commitment") || strings.Contains(desc, "Sole Tenancy") ||
		strings.Contains(desc, "GPU") || strings.Contains(desc, "Hyperdisk") {
		return false
	}
	descLower := strings.ToLower(desc)
	// If it contains both Core and RAM (e.g. "Core and Ram"), it is a predefined monolithic VM, not a standalone component SKU
	if strings.Contains(descLower, "core") && strings.Contains(descLower, "ram") {
		return false
	}
	isCoreOrRAM := strings.Contains(desc, "Instance Core") || strings.Contains(desc, "Instance Ram") || strings.Contains(desc, "Instance RAM") ||
		strings.Contains(desc, "Core running") || strings.Contains(desc, "Ram running") || strings.Contains(desc, "RAM running")
	return isCoreOrRAM
}

func parseGCPComponent(sku gcpSKU) (string, bool, bool) {
	desc := sku.Description
	descLower := strings.ToLower(desc)

	var family string
	switch {
	case strings.Contains(desc, "N2D"):
		family = "n2d"
	case strings.Contains(desc, "N2"):
		family = "n2"
	case strings.Contains(desc, "N1"):
		family = "n1"
	case strings.Contains(desc, "E2"):
		family = "e2"
	case strings.Contains(desc, "C2D"):
		family = "c2d"
	case strings.Contains(desc, "C2") || strings.Contains(descLower, "compute optimized"):
		family = "c2"
	case strings.Contains(desc, "C3"):
		family = "c3"
	case strings.Contains(desc, "T2D"):
		family = "t2d"
	case strings.Contains(desc, "T2A"):
		family = "t2a"
	default:
		return "", false, false
	}

	isCore := strings.Contains(descLower, "core")
	isRAM := strings.Contains(descLower, "ram")
	return family, isCore, isRAM
}

func collectComponentPricing(sku gcpSKU, components map[string]*gcpComponentSpec) {
	if len(sku.PricingInfo) == 0 || len(sku.PricingInfo[0].PricingExpression.TieredRates) == 0 {
		return
	}
	family, isCore, isRAM := parseGCPComponent(sku)
	if family == "" || (!isCore && !isRAM) {
		return
	}
	unitPrice := sku.PricingInfo[0].PricingExpression.TieredRates[0].UnitPrice
	priceAmount, err := extractUnitPrice(unitPrice)
	if err != nil || priceAmount.IsZero() {
		return
	}

	comp, ok := components[family]
	if !ok {
		regions := sku.ServiceRegions
		if len(regions) == 0 {
			regions = []string{"global"}
		}
		currency := unitPrice.CurrencyCode
		if currency == "" {
			currency = "USD"
		}
		unit := sku.PricingInfo[0].PricingExpression.UsageUnit
		if unit == "h" || unit == "hour" {
			unit = "Hrs"
		}
		comp = &gcpComponentSpec{
			unit:     unit,
			currency: currency,
			regions:  regions,
		}
		components[family] = comp
	}

	if isCore {
		comp.corePrice = priceAmount
	}
	if isRAM {
		comp.ramPrice = priceAmount
	}
	if len(sku.ServiceRegions) > 0 {
		comp.regions = sku.ServiceRegions
	}
}

func composeMachineTypePricing(components map[string]*gcpComponentSpec, fetchedAt time.Time, sink quarantine.Sink) ([]domain.PriceObservation, error) {
	var observations []domain.PriceObservation
	for family, comp := range components {
		if comp.corePrice.IsZero() || comp.ramPrice.IsZero() {
			continue
		}
		for machineType, spec := range knownGCPVMSpecs {
			if spec.Family != family {
				continue
			}
			hourlyPrice := comp.corePrice.Mul(decimal.NewFromFloat(spec.VCPU)).Add(comp.ramPrice.Mul(decimal.NewFromFloat(spec.RAMGB)))
			skuID := fmt.Sprintf("SKU-GCP-VM-%s", strings.ToUpper(machineType))

			for _, region := range comp.regions {
				regionGroup, err := regionmap.MapGCPRegion(region)
				if err != nil {
					if errors.Is(err, regionmap.ErrUnmappedRegion) {
						if sink != nil {
							_ = sink.Record(context.Background(), quarantine.UnmappedItem{
								Provider:   "gcp",
								Category:   "compute",
								Kind:       "region",
								RawValue:   region,
								SkuID:      skuID,
								ObservedAt: fetchedAt,
							})
						}
						continue
					}
					return nil, fmt.Errorf("gcp compose region %s: %w", region, err)
				}

				observations = append(observations, domain.PriceObservation{
					Provider:        "gcp",
					ServiceCategory: "compute",
					SkuID:           skuID,
					DisplayName:     machineType,
					Region:          region,
					RegionGroup:     regionGroup,
					Unit:            "Hrs",
					PriceAmount:     hourlyPrice,
					PriceCurrency:   comp.currency,
					PricingModel:    "OnDemand",
					Attributes:      spec,
					FetchedAt:       fetchedAt,
				})
			}
		}
	}
	return observations, nil
}

func isComputeInstance(sku gcpSKU) bool {
	// Must be OnDemand
	if sku.Category.UsageType != "OnDemand" {
		return false
	}

	// Exclude Windows / SQL Server / RHEL
	desc := sku.Description
	if strings.Contains(desc, "Windows") || strings.Contains(desc, "SQL Server") || strings.Contains(desc, "RHEL") || strings.Contains(desc, "SLES") {
		return false
	}

	// Exclude Spot / Preemptible / Commitments
	if strings.Contains(desc, "Spot") || strings.Contains(desc, "Preemptible") || strings.Contains(desc, "Commitment") {
		return false
	}

	return true
}

func isStorageProduct(sku gcpSKU) bool {
	if sku.Category.UsageType != "OnDemand" {
		return false
	}
	if len(sku.PricingInfo) == 0 {
		return false
	}
	usageUnit := sku.PricingInfo[0].PricingExpression.UsageUnit
	// Match monthly storage units (e.g. GiBy.mo, GiBy.month)
	if !strings.Contains(usageUnit, "mo") && !strings.Contains(usageUnit, "month") && !strings.Contains(usageUnit, "Mo") {
		return false
	}

	// Exclude metadata, tag bindings, operations, and auxiliary services
	desc := sku.Description + " " + sku.Category.ResourceGroup + " " + sku.Name
	if strings.Contains(desc, "TagBinding") || strings.Contains(desc, "Tag Binding") ||
		strings.Contains(desc, "Autoclass") || strings.Contains(desc, "Early Delete") ||
		strings.Contains(desc, "Retrieval") || strings.Contains(desc, "Operations Class") ||
		strings.Contains(desc, "Replication") || strings.Contains(desc, "Data Box") {
		return false
	}

	// Must match a recognized storage class keyword
	for _, candidate := range []string{
		"Standard", "Nearline", "Coldline", "Archive", "DRA", "DRAStorage",
		"Regional", "Multi-Regional", "Dual-Region", "Storage",
	} {
		if strings.Contains(desc, candidate) {
			return true
		}
	}
	return false
}

func parseGCPStorageClass(resourceGroup, description, name string) (string, error) {
	for _, text := range []string{resourceGroup, description, name} {
		for _, candidate := range []string{
			"DRAStorage", "DRA", "Standard", "Nearline", "Coldline", "Archive",
			"Regional", "Multi-Regional", "Dual-Region",
		} {
			if strings.Contains(text, candidate) {
				return storageclassmap.MapGCPStorageClass(candidate)
			}
		}
	}
	return storageclassmap.MapGCPStorageClass(resourceGroup)
}

func parseGCPTransferType(description, resourceGroup string) (string, error) {
	for _, text := range []string{description, resourceGroup} {
		if text != "" {
			if tt, err := transfertypemap.MapGCPTransferType(text); err == nil {
				return tt, nil
			}
		}
	}
	return transfertypemap.MapGCPTransferType(description)
}

var gcpMachineTypeRegex = regexp.MustCompile(`(?i)\b([a-z0-9]+)-(standard|highmem|highcpu|micro|small|medium)-?(\d+)?\b`)

func parseGCPAttributes(description, name string) (domain.ComputeAttributes, bool) {
	fullText := description + " " + name
	matches := gcpMachineTypeRegex.FindStringSubmatch(fullText)
	if len(matches) > 0 {
		machineTypeKey := strings.ToLower(matches[0])
		if spec, ok := knownGCPVMSpecs[machineTypeKey]; ok {
			return spec, true
		}
	}
	return domain.ComputeAttributes{}, false
}

func normalizeDatabaseSKU(sku gcpSKU, category string, fetchedAt time.Time, sink quarantine.Sink) ([]domain.PriceObservation, error) {
	if len(sku.PricingInfo) == 0 || len(sku.PricingInfo[0].PricingExpression.TieredRates) == 0 {
		return nil, nil
	}

	unitPrice := sku.PricingInfo[0].PricingExpression.TieredRates[0].UnitPrice
	priceAmount, err := extractUnitPrice(unitPrice)
	if err != nil {
		return nil, fmt.Errorf("gcp normalize sku %s: %w", sku.SkuID, err)
	}
	if priceAmount.IsZero() {
		return nil, nil
	}

	engine, err := parseGCPDatabaseEngine(sku)
	if err != nil {
		if errors.Is(err, databaseenginemap.ErrUnmappedDatabaseEngine) {
			if sink != nil {
				_ = sink.Record(context.Background(), quarantine.UnmappedItem{
					Provider:   "gcp",
					Category:   category,
					Kind:       "database_engine",
					RawValue:   sku.Description,
					SkuID:      sku.SkuID,
					ObservedAt: fetchedAt,
				})
			}
			slog.Warn("gcp normalize: skipping SKU due to unmapped database engine", "sku", sku.SkuID, "desc", sku.Description)
			return nil, nil
		}
		return nil, fmt.Errorf("gcp normalize sku %s: %w", sku.SkuID, err)
	}

	isStorage := strings.Contains(sku.Description, "Storage") ||
		strings.Contains(sku.Category.ResourceGroup, "PD") ||
		strings.Contains(sku.PricingInfo[0].PricingExpression.UsageUnitDescription, "month") ||
		strings.Contains(sku.PricingInfo[0].PricingExpression.UsageUnit, "mo")

	multiAZ := strings.Contains(sku.Description, "Regional") ||
		strings.Contains(sku.Description, "HA") ||
		strings.Contains(sku.Category.ResourceGroup, "Regional")

	regions := sku.ServiceRegions
	if len(regions) == 0 {
		regions = []string{"global"}
	}

	currency := unitPrice.CurrencyCode
	if currency == "" {
		currency = "USD"
	}

	var results []domain.PriceObservation
	for _, region := range regions {
		regionGroup, err := regionmap.MapGCPRegion(region)
		if err != nil {
			if errors.Is(err, regionmap.ErrUnmappedRegion) {
				if sink != nil {
					_ = sink.Record(context.Background(), quarantine.UnmappedItem{
						Provider:   "gcp",
						Category:   category,
						Kind:       "region",
						RawValue:   region,
						SkuID:      sku.SkuID,
						ObservedAt: fetchedAt,
					})
				}
				slog.Warn("gcp normalize: skipping SKU due to unmapped region", "sku", sku.SkuID, "region", region)
				continue
			}
			return nil, fmt.Errorf("gcp normalize sku %s: %w", sku.SkuID, err)
		}

		if isStorage {
			storageFamily := "ssd"
			if strings.Contains(sku.Description, "HDD") || strings.Contains(sku.Category.ResourceGroup, "PDStandard") {
				storageFamily = "hdd"
			}

			results = append(results, domain.PriceObservation{
				Provider:        "gcp",
				ServiceCategory: category,
				SkuID:           sku.SkuID,
				DisplayName:     sku.Description,
				Region:          region,
				RegionGroup:     regionGroup,
				Unit:            "GB-Mo",
				PriceAmount:     priceAmount,
				PriceCurrency:   currency,
				PricingModel:    "OnDemand",
				DatabaseRDBMSAttributes: domain.DatabaseRDBMSAttributes{
					Engine:        engine,
					VCPU:          0,
					RAMGB:         0,
					StorageGB:     1,
					MultiAZ:       multiAZ,
					StorageFamily: storageFamily,
					ComponentType: "storage",
				},
				FetchedAt: fetchedAt,
			})
		} else {
			vcpu, ram, tier := parseGCPDatabaseAttributes(sku.Description, sku.Name)
			unit := sku.PricingInfo[0].PricingExpression.UsageUnit
			if unit == "h" || unit == "hour" {
				unit = "Hrs"
			}

			results = append(results, domain.PriceObservation{
				Provider:        "gcp",
				ServiceCategory: category,
				SkuID:           sku.SkuID,
				DisplayName:     sku.Description,
				Region:          region,
				RegionGroup:     regionGroup,
				Unit:            unit,
				PriceAmount:     priceAmount,
				PriceCurrency:   currency,
				PricingModel:    "OnDemand",
				DatabaseRDBMSAttributes: domain.DatabaseRDBMSAttributes{
					Engine:         engine,
					VCPU:           vcpu,
					RAMGB:          ram,
					StorageGB:      0,
					MultiAZ:        multiAZ,
					DeploymentTier: tier,
					ComponentType:  "instance",
				},
				FetchedAt: fetchedAt,
			})
		}
	}

	return results, nil
}

func parseGCPDatabaseEngine(sku gcpSKU) (string, error) {
	for _, text := range []string{sku.Description, sku.Category.ResourceGroup, sku.Category.ServiceDisplayName, sku.Name} {
		for _, cand := range []string{"PostgreSQL", "Postgres", "AlloyDB", "MySQL", "SQL Server", "SQLServer"} {
			if strings.Contains(text, cand) {
				return databaseenginemap.MapGCPEngine(cand)
			}
		}
	}
	return databaseenginemap.MapGCPEngine(sku.Description)
}

var gcpDBCustomRegex = regexp.MustCompile(`(?i)db-custom-(\d+)-(\d+)`)
var gcpDBVCPURegex = regexp.MustCompile(`(?i)(\d+)\s*vCPU[,\s]+(\d+)\s*GB`)

func parseGCPDatabaseAttributes(description, name string) (float64, float64, string) {
	tier := "standard"
	if strings.Contains(description, "AlloyDB") || strings.Contains(name, "AlloyDB") {
		tier = "alloydb"
	}

	fullText := description + " " + name
	if m := gcpDBCustomRegex.FindStringSubmatch(fullText); len(m) >= 3 {
		vcpu, _ := strconv.ParseFloat(m[1], 64)
		ramMB, _ := strconv.ParseFloat(m[2], 64)
		return vcpu, ramMB / 1024.0, tier
	}

	if m := gcpDBVCPURegex.FindStringSubmatch(fullText); len(m) >= 3 {
		vcpu, _ := strconv.ParseFloat(m[1], 64)
		ram, _ := strconv.ParseFloat(m[2], 64)
		return vcpu, ram, tier
	}

	if attrs, ok := parseGCPAttributes(description, name); ok {
		return attrs.VCPU, attrs.RAMGB, tier
	}

	return 2, 8, tier
}

func normalizeDatabaseNoSQLSKU(sku gcpSKU, category string, fetchedAt time.Time, sink quarantine.Sink) ([]domain.PriceObservation, error) {
	if len(sku.PricingInfo) == 0 || len(sku.PricingInfo[0].PricingExpression.TieredRates) == 0 {
		return nil, nil
	}

	unitPrice := sku.PricingInfo[0].PricingExpression.TieredRates[0].UnitPrice
	priceAmount, err := extractUnitPrice(unitPrice)
	if err != nil {
		return nil, fmt.Errorf("gcp normalize sku %s: %w", sku.SkuID, err)
	}
	if priceAmount.IsZero() {
		return nil, nil
	}

	serviceName := sku.Category.ServiceDisplayName
	if serviceName == "" {
		serviceName = sku.Category.ResourceFamily
	}
	dataModel, err := nosqldatamodelmap.MapGCPDataModel(serviceName)
	if err != nil {
		dataModel, err = nosqldatamodelmap.MapGCPDataModel(sku.Description)
		if err != nil {
			dataModel = nosqldatamodelmap.DataModelDocument
		}
	}

	multiRegion := strings.Contains(sku.Description, "Multi-Region") ||
		strings.Contains(sku.Description, "MultiRegion") ||
		strings.Contains(sku.Category.ResourceGroup, "MultiRegion") ||
		strings.Contains(sku.Category.ResourceGroup, "Multi-Region")

	isStorage := strings.Contains(sku.Description, "Database Stored Data") ||
		strings.Contains(sku.Description, "Storage") ||
		strings.Contains(sku.Category.ResourceGroup, "DatabaseStoredData") ||
		strings.Contains(sku.PricingInfo[0].PricingExpression.UsageUnitDescription, "month") ||
		strings.Contains(sku.PricingInfo[0].PricingExpression.UsageUnit, "mo")

	regions := sku.ServiceRegions
	if len(regions) == 0 {
		regions = []string{"global"}
	}

	currency := unitPrice.CurrencyCode
	if currency == "" {
		currency = "USD"
	}

	var results []domain.PriceObservation
	for _, region := range regions {
		regionGroup, err := regionmap.MapGCPRegion(region)
		if err != nil {
			if errors.Is(err, regionmap.ErrUnmappedRegion) {
				if sink != nil {
					_ = sink.Record(context.Background(), quarantine.UnmappedItem{
						Provider:   "gcp",
						Category:   category,
						Kind:       "region",
						RawValue:   region,
						SkuID:      sku.SkuID,
						ObservedAt: fetchedAt,
					})
				}
				slog.Warn("gcp normalize: skipping SKU due to unmapped region", "sku", sku.SkuID, "region", region)
				continue
			}
			return nil, fmt.Errorf("gcp normalize sku %s: %w", sku.SkuID, err)
		}

		if isStorage {
			results = append(results, domain.PriceObservation{
				Provider:        "gcp",
				ServiceCategory: category,
				SkuID:           sku.SkuID,
				DisplayName:     sku.Description,
				Region:          region,
				RegionGroup:     regionGroup,
				Unit:            "GB-Mo",
				PriceAmount:     priceAmount,
				PriceCurrency:   currency,
				PricingModel:    "OnDemand",
				DatabaseNoSQLAttributes: domain.DatabaseNoSQLAttributes{
					DataModel:     dataModel,
					PricingMode:   "provisioned",
					ReadUnits:     0,
					WriteUnits:    0,
					StorageGB:     1,
					StorageClass:  "standard",
					MultiRegion:   multiRegion,
					ComponentType: "storage",
				},
				FetchedAt: fetchedAt,
			})
		} else {
			var pricingMode = "on_demand"
			var componentType = "request_operations"
			var readUnits float64
			var writeUnits float64

			desc := sku.Description
			resGroup := sku.Category.ResourceGroup

			switch {
			case strings.Contains(desc, "Read") || strings.Contains(resGroup, "Reads") || strings.Contains(resGroup, "Read"):
				readUnits = 100000
			case strings.Contains(desc, "Write") || strings.Contains(resGroup, "Writes") || strings.Contains(resGroup, "Write"):
				writeUnits = 100000
			case strings.Contains(desc, "Delete") || strings.Contains(resGroup, "Deletes") || strings.Contains(resGroup, "Delete"):
				// Delete operations
			default:
				continue
			}

			results = append(results, domain.PriceObservation{
				Provider:        "gcp",
				ServiceCategory: category,
				SkuID:           sku.SkuID,
				DisplayName:     sku.Description,
				Region:          region,
				RegionGroup:     regionGroup,
				Unit:            "100k-ops",
				PriceAmount:     priceAmount,
				PriceCurrency:   currency,
				PricingModel:    "OnDemand",
				DatabaseNoSQLAttributes: domain.DatabaseNoSQLAttributes{
					DataModel:     dataModel,
					PricingMode:   pricingMode,
					ReadUnits:     readUnits,
					WriteUnits:    writeUnits,
					StorageGB:     0,
					MultiRegion:   multiRegion,
					ComponentType: componentType,
				},
				FetchedAt: fetchedAt,
			})
		}
	}

	return results, nil
}

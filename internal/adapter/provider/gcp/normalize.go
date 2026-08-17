package gcp

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"github.com/thatengineerguy21/CloudVitta/internal/adapter/provider"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/catalogmap"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/regionmap"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/storageclassmap"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/transfertypemap"
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
// It skips SKUs with unmapped compute attributes, logging them at warn level.
func Normalize(r io.Reader, fetchedAt time.Time) ([]domain.PriceObservation, string, error) {
	dec := json.NewDecoder(r)

	var observations []domain.PriceObservation
	var nextPageToken string

	// Advance to the first token
	t, err := dec.Token()
	if err != nil {
		if err == io.EOF {
			return nil, "", nil
		}
		return nil, "", fmt.Errorf("gcp normalize: %w", err)
	}

	if delim, ok := t.(json.Delim); !ok || delim != '{' {
		return nil, "", fmt.Errorf("gcp normalize: expected '{' at start of response")
	}

	for dec.More() {
		t, err := dec.Token()
		if err != nil {
			return nil, "", fmt.Errorf("gcp normalize: read key: %w", err)
		}
		key, ok := t.(string)
		if !ok {
			continue
		}

		switch key {
		case "skus":
			t, err = dec.Token()
			if err != nil {
				return nil, "", fmt.Errorf("gcp normalize: read 'skus' value: %w", err)
			}
			if delim, ok := t.(json.Delim); !ok || delim != '[' {
				return nil, "", fmt.Errorf("gcp normalize: expected '[' after 'skus'")
			}

			for dec.More() {
				var sku gcpSKU
				if err := dec.Decode(&sku); err != nil {
					return nil, "", fmt.Errorf("gcp normalize: decode sku: %w", err)
				}

				// Filter 1: Check service category mapping (fails loudly if unmapped)
				serviceName := sku.Category.ServiceDisplayName
				if serviceName == "" {
					serviceName = sku.Category.ResourceFamily
				}
				category, err := catalogmap.MapGCPProduct(serviceName)
				if err != nil {
					return nil, "", fmt.Errorf("gcp normalize sku %s: %w", sku.SkuID, err)
				}

				var skuObs []domain.PriceObservation
				switch category {
				case "compute":
					skuObs, err = normalizeComputeSKU(sku, category, fetchedAt)
				case "storage":
					skuObs, err = normalizeStorageSKU(sku, category, fetchedAt)
				case "network":
					skuObs, err = normalizeNetworkSKU(sku, category, fetchedAt)
				}
				if err != nil {
					return nil, "", err
				}
				observations = append(observations, skuObs...)
			}
			// Consume ']'
			if _, err := dec.Token(); err != nil {
				return nil, "", err
			}
		case "nextPageToken":
			if err := dec.Decode(&nextPageToken); err != nil {
				return nil, "", fmt.Errorf("gcp normalize: decode nextPageToken: %w", err)
			}
		default:
			// Discard other top-level keys
			if err := provider.SkipJSONValue(dec); err != nil {
				return nil, "", fmt.Errorf("gcp normalize: skip key %s: %w", key, err)
			}
		}
	}

	return observations, nextPageToken, nil
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

func normalizeComputeSKU(sku gcpSKU, category string, fetchedAt time.Time) ([]domain.PriceObservation, error) {
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

func normalizeStorageSKU(sku gcpSKU, category string, fetchedAt time.Time) ([]domain.PriceObservation, error) {
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

func normalizeNetworkSKU(sku gcpSKU, category string, fetchedAt time.Time) ([]domain.PriceObservation, error) {
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

	var results []domain.PriceObservation
	for _, region := range regions {
		regionGroup, err := regionmap.MapGCPRegion(region)
		if err != nil {
			return nil, fmt.Errorf("gcp normalize sku %s: %w", sku.SkuID, err)
		}

		transferType, err := parseGCPTransferType(sku.Description, sku.Category.ResourceGroup)
		if err != nil {
			return nil, fmt.Errorf("gcp normalize sku %s transfer type: %w", sku.SkuID, err)
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
	return true
}

func parseGCPStorageClass(resourceGroup, description, name string) (string, error) {
	for _, text := range []string{resourceGroup, description, name} {
		for _, candidate := range []string{"DRAStorage", "DRA", "Standard", "Nearline", "Coldline", "Archive"} {
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

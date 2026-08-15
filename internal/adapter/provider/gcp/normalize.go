package gcp

import (
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/catalogmap"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/regionmap"
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

type gcpCatalogResponse struct {
	Skus          []gcpSKU `json:"skus"`
	NextPageToken string   `json:"nextPageToken"`
}

type gcpVMSpec struct {
	vcpu   float64
	ramGB  float64
	family string
}

var knownGCPVMSpecs = map[string]gcpVMSpec{
	"n2-standard-2":  {vcpu: 2, ramGB: 8, family: "n2"},
	"n2-standard-4":  {vcpu: 4, ramGB: 16, family: "n2"},
	"n2-standard-8":  {vcpu: 8, ramGB: 32, family: "n2"},
	"n2-standard-16": {vcpu: 16, ramGB: 64, family: "n2"},
	"e2-standard-2":  {vcpu: 2, ramGB: 8, family: "e2"},
	"e2-standard-4":  {vcpu: 4, ramGB: 16, family: "e2"},
	"e2-standard-8":  {vcpu: 8, ramGB: 32, family: "e2"},
	"e2-medium":      {vcpu: 2, ramGB: 4, family: "e2"},
	"e2-small":       {vcpu: 2, ramGB: 2, family: "e2"},
	"e2-micro":       {vcpu: 2, ramGB: 1, family: "e2"},
	"c2-standard-4":  {vcpu: 4, ramGB: 16, family: "c2"},
	"c2-standard-8":  {vcpu: 8, ramGB: 32, family: "c2"},
	"n1-standard-1":  {vcpu: 1, ramGB: 3.75, family: "n1"},
	"n1-standard-2":  {vcpu: 2, ramGB: 7.5, family: "n1"},
	"n1-standard-4":  {vcpu: 4, ramGB: 15, family: "n1"},
}

// Normalize parses a GCP Cloud Billing Catalog API JSON stream and returns normalized domain observations.
// It fails loudly if an unmapped product code or region is encountered.
func Normalize(r io.Reader, fetchedAt time.Time) ([]domain.PriceObservation, error) {
	var payload gcpCatalogResponse
	dec := json.NewDecoder(r)
	if err := dec.Decode(&payload); err != nil {
		return nil, fmt.Errorf("gcp normalize: decode JSON: %w", err)
	}

	var observations []domain.PriceObservation

	for _, sku := range payload.Skus {
		// Filter 1: Check service category mapping (fails loudly if unmapped)
		serviceName := sku.Category.ServiceDisplayName
		if serviceName == "" {
			serviceName = sku.Category.ResourceFamily
		}
		category, err := catalogmap.MapGCPProduct(serviceName)
		if err != nil {
			return nil, fmt.Errorf("gcp normalize sku %s: %w", sku.SkuID, err)
		}

		// Filter 2: Compute instance filters (OnDemand / Linux only)
		if !isComputeInstance(sku) {
			continue
		}

		// Extract price from first pricing info and tiered rate
		if len(sku.PricingInfo) == 0 || len(sku.PricingInfo[0].PricingExpression.TieredRates) == 0 {
			continue
		}
		rate := sku.PricingInfo[0].PricingExpression.TieredRates[0]
		unitPrice := rate.UnitPrice

		var unitsDec decimal.Decimal
		if unitPrice.Units != "" {
			var err error
			unitsDec, err = decimal.NewFromString(unitPrice.Units)
			if err != nil {
				return nil, fmt.Errorf("gcp normalize sku %s: invalid units %q: %w", sku.SkuID, unitPrice.Units, err)
			}
		} else {
			unitsDec = decimal.Zero
		}

		nanosDec := decimal.NewFromInt(int64(unitPrice.Nanos)).Div(decimal.NewFromInt(1_000_000_000))
		priceAmount := unitsDec.Add(nanosDec)

		if priceAmount.IsZero() {
			continue
		}

		vcpu, ram, family := parseGCPAttributes(sku.Category.ResourceGroup, sku.Description, sku.Name)

		unit := sku.PricingInfo[0].PricingExpression.UsageUnit
		if unit == "h" || unit == "hour" {
			unit = "Hrs"
		}

		currency := unitPrice.CurrencyCode
		if currency == "" {
			currency = "USD"
		}

		// A GCP SKU can apply to multiple service regions
		regions := sku.ServiceRegions
		if len(regions) == 0 {
			regions = []string{"global"}
		}

		for _, region := range regions {
			regionGroup, err := regionmap.MapGCPRegion(region)
			if err != nil {
				return nil, fmt.Errorf("gcp normalize sku %s: %w", sku.SkuID, err)
			}

			obs := domain.PriceObservation{
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
				Attributes: domain.ComputeAttributes{
					VCPU:   vcpu,
					RAMGB:  ram,
					Family: family,
				},
				FetchedAt: fetchedAt,
			}

			observations = append(observations, obs)
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

	// Exclude Spot / Preemptible
	if strings.Contains(desc, "Spot") || strings.Contains(desc, "Preemptible") {
		return false
	}

	return true
}

var gcpMachineTypeRegex = regexp.MustCompile(`(?i)\b([a-z0-9]+)-(standard|highmem|highcpu|micro|small|medium)-?(\d+)?\b`)

func parseGCPAttributes(resourceGroup, description, name string) (float64, float64, string) {
	// Search for standard machine type in description or name
	fullText := description + " " + name
	matches := gcpMachineTypeRegex.FindStringSubmatch(fullText)
	if len(matches) >= 3 {
		machineTypeKey := strings.ToLower(matches[0])
		if spec, ok := knownGCPVMSpecs[machineTypeKey]; ok {
			return spec.vcpu, spec.ramGB, spec.family
		}

		series := strings.ToLower(matches[1])
		tier := strings.ToLower(matches[2])
		var vcpu float64
		if len(matches) > 3 && matches[3] != "" {
			vcpu, _ = strconv.ParseFloat(matches[3], 64)
		}
		if vcpu == 0 {
			vcpu = 2
		}

		var ram float64
		switch tier {
		case "highmem":
			ram = vcpu * 8
		case "highcpu":
			ram = vcpu * 0.9
		case "micro":
			ram = 1
		case "small":
			ram = 2
		case "medium":
			ram = 4
		default: // standard
			ram = vcpu * 4
		}
		return vcpu, ram, series
	}

	family := strings.ToLower(resourceGroup)
	if family == "" {
		family = "custom"
	}
	return 0, 0, family
}

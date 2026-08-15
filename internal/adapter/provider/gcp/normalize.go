package gcp

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"

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
	UsageUnit   string          `json:"usageUnit"`
	TieredRates []gcpTieredRate `json:"tieredRates"`
}

type gcpPricingInfo struct {
	PricingExpression gcpPricingExpression `json:"pricingExpression"`
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

var knownGCPVMSpecs = map[string]domain.ComputeAttributes{
	"n1-standard-1":  {VCPU: 1, RAMGB: 3.75, Family: "n1"},
	"n1-standard-2":  {VCPU: 2, RAMGB: 7.5, Family: "n1"},
	"n1-standard-4":  {VCPU: 4, RAMGB: 15, Family: "n1"},
	"n1-standard-8":  {VCPU: 8, RAMGB: 30, Family: "n1"},
	"n1-standard-16": {VCPU: 16, RAMGB: 60, Family: "n1"},

	"n2-standard-2":  {VCPU: 2, RAMGB: 8, Family: "n2"},
	"n2-standard-4":  {VCPU: 4, RAMGB: 16, Family: "n2"},
	"n2-standard-8":  {VCPU: 8, RAMGB: 32, Family: "n2"},
	"n2-standard-16": {VCPU: 16, RAMGB: 64, Family: "n2"},

	"n2d-standard-2":  {VCPU: 2, RAMGB: 8, Family: "n2d"},
	"n2d-standard-4":  {VCPU: 4, RAMGB: 16, Family: "n2d"},
	"n2d-standard-8":  {VCPU: 8, RAMGB: 32, Family: "n2d"},
	"n2d-standard-16": {VCPU: 16, RAMGB: 64, Family: "n2d"},

	"e2-micro":       {VCPU: 2, RAMGB: 1, Family: "e2"},
	"e2-small":       {VCPU: 2, RAMGB: 2, Family: "e2"},
	"e2-medium":      {VCPU: 2, RAMGB: 4, Family: "e2"},
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
						return nil, "", fmt.Errorf("gcp normalize sku %s: invalid units %q: %w", sku.SkuID, unitPrice.Units, err)
					}
				} else {
					unitsDec = decimal.Zero
				}

				nanosDec := decimal.NewFromInt(int64(unitPrice.Nanos)).Div(decimal.NewFromInt(1_000_000_000))
				priceAmount := unitsDec.Add(nanosDec)

				if priceAmount.IsZero() {
					continue
				}

				attrs, ok := parseGCPAttributes(sku.Description, sku.Name)
				if !ok {
					slog.Warn("gcp normalize: skipping SKU due to unmapped machine type", "provider", "gcp", "sku", sku.SkuID, "description", sku.Description, "name", sku.Name)
					continue
				}

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
						return nil, "", fmt.Errorf("gcp normalize sku %s: %w", sku.SkuID, err)
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
						Attributes:      attrs,
						FetchedAt:       fetchedAt,
					}

					observations = append(observations, obs)
				}
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
			// Skip unknown fields
			var dummy interface{}
			if err := dec.Decode(&dummy); err != nil {
				return nil, "", fmt.Errorf("gcp normalize: decode dummy: %w", err)
			}
		}
	}

	// Consume '}'
	if _, err := dec.Token(); err != nil {
		return nil, "", err
	}

	return observations, nextPageToken, nil
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

func parseGCPAttributes(description, name string) (domain.ComputeAttributes, bool) {
	fullText := strings.ToLower(description + " " + name)
	for key, spec := range knownGCPVMSpecs {
		if strings.Contains(fullText, key) {
			return spec, true
		}
	}
	return domain.ComputeAttributes{}, false
}

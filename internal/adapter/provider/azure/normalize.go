package azure

import (
	"encoding/json"
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
	"github.com/thatengineerguy21/CloudVitta/internal/matching/storageclassmap"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/transfertypemap"
)

type azureItem struct {
	CurrencyCode       string      `json:"currencyCode"`
	TierMinimumUnits   float64     `json:"tierMinimumUnits"`
	RetailPrice        json.Number `json:"retailPrice"`
	UnitPrice          json.Number `json:"unitPrice"`
	ArmRegionName      string      `json:"armRegionName"`
	Location           string      `json:"location"`
	EffectiveStartDate string      `json:"effectiveStartDate"`
	MeterID            string      `json:"meterId"`
	MeterName          string      `json:"meterName"`
	ProductID          string      `json:"productId"`
	SkuID              string      `json:"skuId"`
	ProductName        string      `json:"productName"`
	SkuName            string      `json:"skuName"`
	ServiceID          string      `json:"serviceId"`
	ServiceName        string      `json:"serviceName"`
	ServiceFamily      string      `json:"serviceFamily"`
	UnitOfMeasure      string      `json:"unitOfMeasure"`
	Type               string      `json:"type"`
	IsPrimaryMeter     bool        `json:"isPrimaryMeterRegion"`
	ArmSkuName         string      `json:"armSkuName"`
}

type azurePriceListResponse struct {
	BillingCurrency    string      `json:"BillingCurrency"`
	CustomerEntityID   string      `json:"CustomerEntityId"`
	CustomerEntityType string      `json:"CustomerEntityType"`
	Items              []azureItem `json:"Items"`
	NextPageLink       string      `json:"NextPageLink"`
	Count              int         `json:"Count"`
}

type azureVMSpec struct {
	vcpu   float64
	ramGB  float64
	family string
}

var knownAzureVMSpecs = map[string]azureVMSpec{
	"Standard_B1s":     {vcpu: 1, ramGB: 1, family: "b"},
	"Standard_B1ms":    {vcpu: 1, ramGB: 2, family: "b"},
	"Standard_B2s":     {vcpu: 2, ramGB: 4, family: "b"},
	"Standard_B2ms":    {vcpu: 2, ramGB: 8, family: "b"},
	"Standard_B4ms":    {vcpu: 4, ramGB: 16, family: "b"},
	"Standard_B8ms":    {vcpu: 8, ramGB: 32, family: "b"},
	"Standard_D2s_v3":  {vcpu: 2, ramGB: 8, family: "d"},
	"Standard_D4s_v3":  {vcpu: 4, ramGB: 16, family: "d"},
	"Standard_D8s_v3":  {vcpu: 8, ramGB: 32, family: "d"},
	"Standard_D16s_v3": {vcpu: 16, ramGB: 64, family: "d"},
	"Standard_D32s_v3": {vcpu: 32, ramGB: 128, family: "d"},
	"Standard_D2_v3":   {vcpu: 2, ramGB: 8, family: "d"},
	"Standard_D4_v3":   {vcpu: 4, ramGB: 16, family: "d"},
	"Standard_D8_v3":   {vcpu: 8, ramGB: 32, family: "d"},
	"Standard_D2s_v4":  {vcpu: 2, ramGB: 8, family: "d"},
	"Standard_D4s_v4":  {vcpu: 4, ramGB: 16, family: "d"},
	"Standard_D8s_v4":  {vcpu: 8, ramGB: 32, family: "d"},
	"Standard_D2s_v5":  {vcpu: 2, ramGB: 8, family: "d"},
	"Standard_D4s_v5":  {vcpu: 4, ramGB: 16, family: "d"},
	"Standard_D8s_v5":  {vcpu: 8, ramGB: 32, family: "d"},
	"Standard_E2s_v3":  {vcpu: 2, ramGB: 16, family: "e"},
	"Standard_E4s_v3":  {vcpu: 4, ramGB: 32, family: "e"},
	"Standard_E8s_v3":  {vcpu: 8, ramGB: 64, family: "e"},
	"Standard_F2s_v2":  {vcpu: 2, ramGB: 4, family: "f"},
	"Standard_F4s_v2":  {vcpu: 4, ramGB: 8, family: "f"},
	"Standard_F8s_v2":  {vcpu: 8, ramGB: 16, family: "f"},
}

// Normalize parses an Azure Retail Prices API JSON stream and returns normalized domain observations and the next page link.
// It fails loudly if an unmapped product code or region is encountered.
func Normalize(r io.Reader, fetchedAt time.Time) ([]domain.PriceObservation, string, error) {
	var payload azurePriceListResponse
	dec := json.NewDecoder(r)
	if err := dec.Decode(&payload); err != nil {
		return nil, "", fmt.Errorf("azure normalize: decode JSON: %w", err)
	}

	var observations []domain.PriceObservation

	for _, item := range payload.Items {
		// Filter 1: Must be Consumption (On-Demand)
		if item.Type != "Consumption" {
			continue
		}

		// Filter 2: Check service category mapping (fails loudly if unmapped)
		category, err := catalogmap.MapAzureProduct(item.ServiceName)
		if err != nil {
			return nil, "", fmt.Errorf("azure normalize sku %s: %w", item.SkuID, err)
		}

		// Filter 3: Check region mapping (fails loudly if unmapped)
		region := item.ArmRegionName
		if region == "" {
			region = item.Location
		}
		regionGroup, err := regionmap.MapAzureRegion(region)
		if err != nil {
			return nil, "", fmt.Errorf("azure normalize sku %s: %w", item.SkuID, err)
		}

		priceAmount, err := decimal.NewFromString(item.UnitPrice.String())
		if err != nil {
			return nil, "", fmt.Errorf("azure normalize sku %s: invalid unit price %q: %w", item.SkuID, item.UnitPrice, err)
		}

		skuID := item.SkuID
		if skuID == "" {
			skuID = item.MeterID
		}

		if category == "compute" {
			if !isComputeInstance(item) {
				continue
			}
			vcpu, ram, family := parseAzureAttributes(item.ArmSkuName, item.SkuName, item.ProductName)

			displayName := item.ArmSkuName
			if displayName == "" {
				displayName = item.SkuName
			}

			unit := item.UnitOfMeasure
			if unit == "1 Hour" || unit == "1 hour" {
				unit = "Hrs"
			}

			obs := domain.PriceObservation{
				Provider:        "azure",
				ServiceCategory: category,
				SkuID:           skuID,
				DisplayName:     displayName,
				Region:          region,
				RegionGroup:     regionGroup,
				Unit:            unit,
				PriceAmount:     priceAmount,
				PriceCurrency:   item.CurrencyCode,
				PricingModel:    "OnDemand",
				Attributes: domain.ComputeAttributes{
					VCPU:   vcpu,
					RAMGB:  ram,
					Family: family,
				},
				FetchedAt: fetchedAt,
			}
			observations = append(observations, obs)

		} else if category == "storage" {
			storageClass, err := parseAzureStorageClass(item.SkuName, item.MeterName, item.ProductName)
			if err != nil {
				return nil, "", fmt.Errorf("azure normalize sku %s: %w", item.SkuID, err)
			}

			displayName := item.ProductName
			if displayName == "" {
				displayName = item.MeterName
			}

			unit := item.UnitOfMeasure
			if strings.EqualFold(unit, "1 GB/Month") || strings.EqualFold(unit, "1 GB/month") || strings.EqualFold(unit, "1 GB/Mo") {
				unit = "GB-Mo"
			}

			obs := domain.PriceObservation{
				Provider:        "azure",
				ServiceCategory: category,
				SkuID:           skuID,
				DisplayName:     displayName,
				Region:          region,
				RegionGroup:     regionGroup,
				Unit:            unit,
				PriceAmount:     priceAmount,
				PriceCurrency:   item.CurrencyCode,
				PricingModel:    "OnDemand",
				StorageAttributes: domain.StorageAttributes{
					SizeGB:       1,
					StorageClass: storageClass,
				},
				FetchedAt: fetchedAt,
			}
			observations = append(observations, obs)

		} else if category == "network" {
			// Tiered Pricing Detection (PRD §16.1):
			// If an item has TierMinimumUnits > 0 (starts after initial tier), skip it.
			if item.TierMinimumUnits > 0 {
				slog.Debug("skipping Azure SKU due to tiered pricing", "provider", "azure", "sku", skuID, "tierMinimumUnits", item.TierMinimumUnits, "reason", "tiered_pricing_not_supported_in_v1")
				continue
			}

			displayName := item.ProductName
			if displayName == "" {
				displayName = item.MeterName
			}

			unit := item.UnitOfMeasure
			if strings.EqualFold(unit, "1 GB") || strings.EqualFold(unit, "1 GB/Month") || strings.EqualFold(unit, "GB") {
				unit = "GB"
			}

			transferType, err := parseAzureTransferType(displayName, item.MeterName, item.SkuName)
			if err != nil {
				return nil, "", fmt.Errorf("azure normalize sku %s transfer type: %w", skuID, err)
			}

			obs := domain.PriceObservation{
				Provider:        "azure",
				ServiceCategory: category,
				SkuID:           skuID,
				DisplayName:     displayName,
				Region:          region,
				RegionGroup:     regionGroup,
				Unit:            unit,
				PriceAmount:     priceAmount,
				PriceCurrency:   item.CurrencyCode,
				PricingModel:    "OnDemand",
				NetworkAttributes: domain.NetworkAttributes{
					EgressGB:     1,
					TransferType: transferType,
				},
				FetchedAt: fetchedAt,
			}
			observations = append(observations, obs)
		}
	}

	return observations, payload.NextPageLink, nil
}

func isComputeInstance(item azureItem) bool {
	// Exclude Windows
	if strings.Contains(item.ProductName, "Windows") || strings.Contains(item.SkuName, "Windows") || strings.Contains(item.MeterName, "Windows") {
		return false
	}

	// Exclude Spot / Low Priority
	if strings.Contains(item.ProductName, "Spot") || strings.Contains(item.SkuName, "Spot") || strings.Contains(item.MeterName, "Spot") {
		return false
	}
	if strings.Contains(item.ProductName, "Low Priority") || strings.Contains(item.SkuName, "Low Priority") || strings.Contains(item.MeterName, "Low Priority") {
		return false
	}

	// Must have a valid size identifier
	if item.ArmSkuName == "" && item.SkuName == "" {
		return false
	}

	return true
}

func parseAzureStorageClass(skuName, meterName, productName string) (string, error) {
	for _, text := range []string{skuName, meterName, productName} {
		for _, candidate := range []string{"Hot", "Standard", "Cool", "Cold", "Archive"} {
			if strings.Contains(text, candidate) {
				return storageclassmap.MapAzureStorageClass(candidate)
			}
		}
	}
	return storageclassmap.MapAzureStorageClass(skuName)
}

func parseAzureTransferType(displayName, meterName, skuName string) (string, error) {
	for _, candidate := range []string{displayName, meterName, skuName} {
		if candidate != "" {
			if tt, err := transfertypemap.MapAzureTransferType(candidate); err == nil {
				return tt, nil
			}
		}
	}
	return transfertypemap.MapAzureTransferType(displayName)
}

var vmSizeRegex = regexp.MustCompile(`(?i)(?:Standard_)?([A-Za-z]+)(\d+)(?:[A-Za-z]*)?(?:_v(\d+))?`)

func parseAzureAttributes(armSkuName, skuName, _ string) (float64, float64, string) {
	// Check known specs first
	if spec, ok := knownAzureVMSpecs[armSkuName]; ok {
		return spec.vcpu, spec.ramGB, spec.family
	}

	nameToParse := armSkuName
	if nameToParse == "" {
		nameToParse = skuName
	}
	nameToParse = strings.TrimSpace(nameToParse)

	matches := vmSizeRegex.FindStringSubmatch(nameToParse)
	if len(matches) >= 3 {
		series := strings.ToLower(matches[1])
		vcpu, err := strconv.ParseFloat(matches[2], 64)
		if err == nil && vcpu > 0 {
			var ram float64
			switch series {
			case "e", "m":
				ram = vcpu * 8
			case "f":
				ram = vcpu * 2
			case "b":
				ram = vcpu * 2
			default: // d, a, etc.
				ram = vcpu * 4
			}
			return vcpu, ram, series
		}
	}

	return 0, 0, strings.ToLower(nameToParse)
}

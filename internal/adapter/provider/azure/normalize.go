package azure

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
	"github.com/thatengineerguy21/CloudVitta/internal/matching/databaseenginemap"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/regionmap"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/storageclassmap"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/transfertypemap"
	"github.com/thatengineerguy21/CloudVitta/internal/quarantine"
)

type azureItem struct {
	CurrencyCode         string      `json:"currencyCode"`
	TierMinimumUnits     float64     `json:"tierMinimumUnits"`
	RetailPrice          json.Number `json:"retailPrice"`
	UnitPrice            json.Number `json:"unitPrice"`
	ArmRegionName        string      `json:"armRegionName"`
	Location             string      `json:"location"`
	EffectiveStartDate   string      `json:"effectiveStartDate"`
	MeterID              string      `json:"meterId"`
	MeterName            string      `json:"meterName"`
	ProductID            string      `json:"productId"`
	SkuID                string      `json:"skuId"`
	ProductName          string      `json:"productName"`
	SkuName              string      `json:"skuName"`
	ServiceName          string      `json:"serviceName"`
	ServiceID            string      `json:"serviceId"`
	ServiceFamily        string      `json:"serviceFamily"`
	UnitOfMeasure        string      `json:"unitOfMeasure"`
	Type                 string      `json:"type"`
	IsPrimaryMeterRegion bool        `json:"isPrimaryMeterRegion"`
	ArmSkuName           string      `json:"armSkuName"`
}

type azurePriceListResponse struct {
	Items        []azureItem `json:"Items"`
	NextPageLink string      `json:"NextPageLink"`
	Count        int         `json:"Count"`
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
	"Standard_D4ds_v4": {vcpu: 4, ramGB: 16, family: "d"},
	"Standard_D2ds_v4": {vcpu: 2, ramGB: 8, family: "d"},
	"Standard_D8ds_v4": {vcpu: 8, ramGB: 32, family: "d"},
	"GP_Gen5_2":        {vcpu: 2, ramGB: 10.2, family: "gp"},
	"GP_Gen5_4":        {vcpu: 4, ramGB: 20.4, family: "gp"},
	"GP_Gen5_8":        {vcpu: 8, ramGB: 40.8, family: "gp"},
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
// Unmapped taxonomy values are recorded to the optional quarantine sink and skipped without aborting the page.
func Normalize(r io.Reader, fetchedAt time.Time, sinks ...quarantine.Sink) ([]domain.PriceObservation, string, error) {
	var sink quarantine.Sink
	if len(sinks) > 0 {
		sink = sinks[0]
	}

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

		skuID := item.SkuID
		if skuID == "" {
			skuID = item.MeterID
		}

		// Filter 2: Check service category mapping (fails loudly if unmapped)
		category, err := catalogmap.MapAzureProduct(item.ServiceName)
		if err != nil {
			if errors.Is(err, catalogmap.ErrUnmappedProduct) {
				if sink != nil {
					_ = sink.Record(context.Background(), quarantine.UnmappedItem{
						Provider:   "azure",
						Category:   "unknown",
						Kind:       "product",
						RawValue:   item.ServiceName,
						SkuID:      skuID,
						ObservedAt: fetchedAt,
					})
				}
				slog.Warn("azure normalize: skipping SKU due to unmapped product", "sku", skuID, "product", item.ServiceName)
				continue
			}
			return nil, "", fmt.Errorf("azure normalize sku %s: %w", item.SkuID, err)
		}

		// Filter 3: Check region mapping (fails loudly if unmapped)
		region := item.ArmRegionName
		if region == "" {
			region = item.Location
		}
		regionGroup, err := regionmap.MapAzureRegion(region)
		if err != nil {
			if errors.Is(err, regionmap.ErrUnmappedRegion) {
				if sink != nil {
					_ = sink.Record(context.Background(), quarantine.UnmappedItem{
						Provider:   "azure",
						Category:   category,
						Kind:       "region",
						RawValue:   region,
						SkuID:      skuID,
						ObservedAt: fetchedAt,
					})
				}
				slog.Warn("azure normalize: skipping SKU due to unmapped region", "sku", skuID, "region", region)
				continue
			}
			return nil, "", fmt.Errorf("azure normalize sku %s: %w", item.SkuID, err)
		}

		priceAmount, err := decimal.NewFromString(item.UnitPrice.String())
		if err != nil {
			return nil, "", fmt.Errorf("azure normalize sku %s: invalid unit price %q: %w", item.SkuID, item.UnitPrice, err)
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
			if !isAzureStorageProduct(item) {
				continue
			}
			storageClass, err := parseAzureStorageClass(item.SkuName, item.MeterName, item.ProductName)
			if err != nil {
				if errors.Is(err, storageclassmap.ErrUnmappedStorageClass) {
					if sink != nil {
						_ = sink.Record(context.Background(), quarantine.UnmappedItem{
							Provider:   "azure",
							Category:   category,
							Kind:       "storage_class",
							RawValue:   item.SkuName,
							SkuID:      skuID,
							ObservedAt: fetchedAt,
						})
					}
					slog.Warn("azure normalize: skipping SKU due to unmapped storage class", "sku", skuID, "sku_name", item.SkuName, "meter_name", item.MeterName)
					continue
				}
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
				if errors.Is(err, transfertypemap.ErrUnmappedTransferType) {
					if sink != nil {
						_ = sink.Record(context.Background(), quarantine.UnmappedItem{
							Provider:   "azure",
							Category:   category,
							Kind:       "transfer_type",
							RawValue:   displayName,
							SkuID:      skuID,
							ObservedAt: fetchedAt,
						})
					}
					slog.Warn("azure normalize: skipping SKU due to unmapped transfer type", "sku", skuID, "display_name", displayName)
					continue
				}
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

		} else if category == "database_rdbms" {
			engine, err := databaseenginemap.MapAzureEngine(item.ServiceName)
			if err != nil {
				if errors.Is(err, databaseenginemap.ErrUnmappedDatabaseEngine) {
					if sink != nil {
						_ = sink.Record(context.Background(), quarantine.UnmappedItem{
							Provider:   "azure",
							Category:   category,
							Kind:       "database_engine",
							RawValue:   item.ServiceName,
							SkuID:      skuID,
							ObservedAt: fetchedAt,
						})
					}
					slog.Warn("azure normalize: skipping SKU due to unmapped database engine", "sku", skuID, "service", item.ServiceName)
					continue
				}
				return nil, "", fmt.Errorf("azure normalize sku %s database engine: %w", skuID, err)
			}

			isStorage := strings.Contains(item.MeterName, "Storage") ||
				strings.Contains(item.SkuName, "Storage") ||
				strings.Contains(item.ProductName, "Storage") ||
				strings.EqualFold(item.UnitOfMeasure, "1 GB/Month") ||
				strings.EqualFold(item.UnitOfMeasure, "1 GB/month") ||
				strings.EqualFold(item.UnitOfMeasure, "1 GB/Mo")

			multiAZ := strings.Contains(item.MeterName, "Zone Redundant") ||
				strings.Contains(item.SkuName, "Zone Redundant") ||
				strings.Contains(item.MeterName, "High Availability")

			if isStorage {
				displayName := item.ProductName
				if displayName == "" {
					displayName = item.MeterName
				}
				storageFamily := "ssd"
				if strings.Contains(strings.ToLower(item.MeterName), "premium") {
					storageFamily = "io1"
				}

				obs := domain.PriceObservation{
					Provider:        "azure",
					ServiceCategory: category,
					SkuID:           skuID,
					DisplayName:     displayName,
					Region:          region,
					RegionGroup:     regionGroup,
					Unit:            "GB-Mo",
					PriceAmount:     priceAmount,
					PriceCurrency:   item.CurrencyCode,
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
				}
				observations = append(observations, obs)
			} else {
				vcpu, ram, _ := parseAzureDatabaseAttributes(item.ArmSkuName, item.SkuName, item.MeterName)
				displayName := item.ArmSkuName
				if displayName == "" {
					displayName = item.SkuName
				}
				if displayName == "" {
					displayName = item.MeterName
				}

				tier := "standard"
				if strings.Contains(strings.ToLower(item.ArmSkuName), "b") || strings.Contains(strings.ToLower(item.SkuName), "burstable") {
					tier = "burstable"
				} else if strings.Contains(strings.ToLower(item.ProductName), "flexible") {
					tier = "flexible"
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
				}
				observations = append(observations, obs)
			}
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

func isAzureStorageProduct(item azureItem) bool {
	// Exclude auxiliary operational/management services
	desc := item.ProductName + " " + item.MeterName + " " + item.SkuName
	if strings.Contains(desc, "Storage Tasks") || strings.Contains(desc, "Operations") || strings.Contains(desc, "Transactions") || strings.Contains(desc, "Storage Mover") || strings.Contains(desc, "Data Box") {
		return false
	}
	unit := strings.ToLower(item.UnitOfMeasure)
	if !strings.Contains(unit, "month") && !strings.Contains(unit, "mo") {
		return false
	}
	return true
}

func parseAzureStorageClass(skuName, meterName, productName string) (string, error) {
	for _, candidate := range []string{skuName, meterName, productName} {
		if candidate != "" {
			if sc, err := storageclassmap.MapAzureStorageClass(candidate); err == nil {
				return sc, nil
			}
		}
	}
	for _, text := range []string{skuName, meterName, productName} {
		for _, candidate := range []string{
			"Hot", "Standard", "Premium", "Cool", "Cold", "Archive",
			"SSD ZRS", "SSD LRS", "SSD", "HDD", "Premium LRS", "Premium ZRS",
			"Standard LRS", "Standard ZRS", "Standard GRS", "Standard GZRS",
			"Account Encrypted GZRS", "Account Encrypted GRS", "Account Encrypted ZRS", "Account Encrypted LRS", "Account Encrypted",
			"GZRS", "GRS", "ZRS", "LRS", "RA-GRS", "RA-GZRS",
			"Blob", "Block Blob", "Page Blob", "Append Blob", "Files", "Disks", "Managed Disks",
			"Premium Files", "Ultra Disks", "Premium SSD v2",
		} {
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
	for _, text := range []string{displayName, meterName, skuName} {
		for _, candidate := range []string{
			"Inter-Region", "Intra-Region", "Internet", "Data Transfer Out", "Bandwidth",
			"Rtn Preference: MGN", "Rtn Preference: Transit", "Routing Preference",
			"ExpressRoute", "Global",
		} {
			if strings.Contains(text, candidate) {
				if tt, err := transfertypemap.MapAzureTransferType(candidate); err == nil {
					return tt, nil
				}
			}
		}
	}
	return transfertypemap.MapAzureTransferType(displayName)
}

var vmSizeRegex = regexp.MustCompile(`(?i)(?:(?:Standard|Basic|Promo)_)?([A-Za-z]+)(\d+)(?:[A-Za-z]*)?(?:_v(\d+))?`)

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

var vcoreRegex = regexp.MustCompile(`(?i)(\d+)\s*vCore`)

func parseAzureDatabaseAttributes(armSkuName, skuName, meterName string) (float64, float64, string) {
	if spec, ok := knownAzureVMSpecs[armSkuName]; ok {
		return spec.vcpu, spec.ramGB, spec.family
	}

	for _, s := range []string{meterName, skuName, armSkuName} {
		m := vcoreRegex.FindStringSubmatch(s)
		if len(m) > 1 {
			if v, err := strconv.ParseFloat(m[1], 64); err == nil && v > 0 {
				return v, v * 4, "general_purpose"
			}
		}
	}

	return 2, 8, "general_purpose"
}

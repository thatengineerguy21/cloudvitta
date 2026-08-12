package aws

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/catalogmap"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/regionmap"
)

type awsProduct struct {
	SKU           string            `json:"sku"`
	ProductFamily string            `json:"productFamily"`
	Attributes    map[string]string `json:"attributes"`
}

type awsPriceDimension struct {
	Unit         string            `json:"unit"`
	PricePerUnit map[string]string `json:"pricePerUnit"`
}

type awsOfferTerm struct {
	PriceDimensions map[string]awsPriceDimension `json:"priceDimensions"`
}

type awsPriceListPayload struct {
	OfferCode string                                        `json:"offerCode"`
	Products  map[string]awsProduct                         `json:"products"`
	Terms     map[string]map[string]map[string]awsOfferTerm `json:"terms"`
}

// Normalize parses an AWS Price List JSON stream and returns normalized domain observations.
// It fails loudly if an unmapped product code or region is encountered.
func Normalize(r io.Reader, fetchedAt time.Time) ([]domain.PriceObservation, error) {
	var payload awsPriceListPayload
	decoder := json.NewDecoder(r)
	if err := decoder.Decode(&payload); err != nil {
		return nil, fmt.Errorf("aws normalize: decode JSON: %w", err)
	}

	onDemandTerms, ok := payload.Terms["OnDemand"]
	if !ok {
		return nil, fmt.Errorf("aws normalize: missing OnDemand terms")
	}

	var observations []domain.PriceObservation

	for sku, product := range payload.Products {
		attrs := product.Attributes
		if attrs == nil {
			continue
		}

		// Filter for EC2 compute instances
		if !isComputeInstance(product, attrs) {
			continue
		}

		// Check service category mapping (fail loudly if unmapped)
		serviceCode := attrs["servicecode"]
		if serviceCode == "" {
			serviceCode = payload.OfferCode
		}
		category, err := catalogmap.MapAWSProduct(serviceCode)
		if err != nil {
			return nil, fmt.Errorf("aws normalize sku %s: %w", sku, err)
		}

		// Check region mapping (fail loudly if unmapped)
		location := attrs["location"]
		if location == "" {
			location = attrs["regionCode"]
		}
		regionGroup, err := regionmap.MapAWSRegion(location)
		if err != nil {
			return nil, fmt.Errorf("aws normalize sku %s: %w", sku, err)
		}

		skuTerms, hasTerms := onDemandTerms[sku]
		if !hasTerms {
			continue
		}

		for _, term := range skuTerms {
			for _, dim := range term.PriceDimensions {
				usdPriceStr, hasUSD := dim.PricePerUnit["USD"]
				if !hasUSD {
					continue
				}

				priceAmount, err := decimal.NewFromString(usdPriceStr)
				if err != nil {
					return nil, fmt.Errorf("aws normalize sku %s: invalid price %q: %w", sku, usdPriceStr, err)
				}

				instanceType := attrs["instanceType"]
				vcpu := parseVCPU(attrs["vcpu"])
				ram := parseRAMGB(attrs["memory"])
				family := parseFamily(instanceType)

				region := attrs["regionCode"]
				if region == "" {
					region = attrs["location"]
				}

				obs := domain.PriceObservation{
					Provider:        "aws",
					ServiceCategory: category,
					SkuID:           sku,
					DisplayName:     instanceType,
					Region:          region,
					RegionGroup:     regionGroup,
					Unit:            dim.Unit,
					PriceAmount:     priceAmount,
					PriceCurrency:   "USD",
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
	}

	return observations, nil
}

func isComputeInstance(product awsProduct, attrs map[string]string) bool {
	// Tenancy filter
	tenancy := attrs["tenancy"]
	if tenancy != "Shared" {
		return false
	}

	// Operating system filter
	os := attrs["operatingSystem"]
	if os != "Linux" && os != "Linux/UNIX" {
		return false
	}

	// Pre-installed software filter
	preInstalled := attrs["preInstalledSw"]
	if preInstalled != "NA" && preInstalled != "" && preInstalled != "None" {
		return false
	}

	// Capacity status filter
	capStatus := attrs["capacitystatus"]
	if capStatus != "" && capStatus != "Used" {
		return false
	}

	// Instance type must be present
	if attrs["instanceType"] == "" {
		return false
	}

	return true
}

func parseVCPU(s string) float64 {
	val, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0
	}
	return val
}

func parseRAMGB(s string) float64 {
	cleaned := strings.ReplaceAll(s, "GiB", "")
	cleaned = strings.ReplaceAll(cleaned, ",", "")
	cleaned = strings.TrimSpace(cleaned)

	val, err := strconv.ParseFloat(cleaned, 64)
	if err != nil {
		return 0
	}
	return val
}

func parseFamily(instanceType string) string {
	parts := strings.Split(instanceType, ".")
	if len(parts) > 0 {
		return parts[0]
	}
	return instanceType
}

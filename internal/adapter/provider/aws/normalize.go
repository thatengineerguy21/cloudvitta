package aws

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"github.com/thatengineerguy21/CloudVitta/internal/adapter/provider"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/catalogmap"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/regionmap"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/storageclassmap"
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

type awsProductMeta struct {
	sku          string
	category     string
	regionGroup  string
	region       string
	displayName  string
	computeAttrs domain.ComputeAttributes
	storageAttrs domain.StorageAttributes
}

// Normalize parses an AWS Price List JSON stream and returns normalized domain observations.
// It streams tokens using json.Decoder with depth-aware skipping to minimize memory overhead
// on large provider files. It enforces that products precede terms, returning ErrPermanentFailure
// if the payload order violates this streaming invariant.
// It fails loudly if an unmapped product code or region is encountered.
func Normalize(r io.Reader, fetchedAt time.Time) ([]domain.PriceObservation, error) {
	dec := json.NewDecoder(r)

	// Consume opening '{'
	t, err := dec.Token()
	if err != nil {
		return nil, fmt.Errorf("aws normalize: read start token: %w", err)
	}
	if delim, ok := t.(json.Delim); !ok || delim != '{' {
		return nil, fmt.Errorf("aws normalize: expected '{' at root, got %v", t)
	}

	var offerCode string
	var productsParsed bool
	filteredProducts := make(map[string]awsProductMeta)
	var observations []domain.PriceObservation

	for dec.More() {
		keyToken, err := dec.Token()
		if err != nil {
			return nil, fmt.Errorf("aws normalize: read top-level key: %w", err)
		}
		key, ok := keyToken.(string)
		if !ok {
			return nil, fmt.Errorf("aws normalize: expected string key, got %v", keyToken)
		}

		switch key {
		case "offerCode":
			if err := dec.Decode(&offerCode); err != nil {
				return nil, fmt.Errorf("aws normalize: decode offerCode: %w", err)
			}

		case "products":
			// Consume opening '{' of products map
			if err := consumeDelim(dec, '{'); err != nil {
				return nil, fmt.Errorf("aws normalize: products open delim: %w", err)
			}

			for dec.More() {
				skuToken, err := dec.Token()
				if err != nil {
					return nil, fmt.Errorf("aws normalize: read product sku: %w", err)
				}
				sku := fmt.Sprintf("%v", skuToken)

				var prod awsProduct
				if err := dec.Decode(&prod); err != nil {
					return nil, fmt.Errorf("aws normalize: decode product %s: %w", sku, err)
				}

				serviceCode := prod.Attributes["servicecode"]
				if serviceCode == "" {
					serviceCode = offerCode
				}

				if isComputeInstance(prod, prod.Attributes) {
					category, err := catalogmap.MapAWSProduct(serviceCode)
					if err != nil {
						return nil, fmt.Errorf("aws normalize sku %s: %w", sku, err)
					}

					location := prod.Attributes["location"]
					if location == "" {
						location = prod.Attributes["regionCode"]
					}
					regionGroup, err := regionmap.MapAWSRegion(location)
					if err != nil {
						return nil, fmt.Errorf("aws normalize sku %s: %w", sku, err)
					}

					instanceType := prod.Attributes["instanceType"]
					vcpu := parseVCPU(prod.Attributes["vcpu"])
					ram := parseRAMGB(prod.Attributes["memory"])
					family := parseFamily(instanceType)

					region := prod.Attributes["regionCode"]
					if region == "" {
						region = prod.Attributes["location"]
					}

					meta := awsProductMeta{
						sku:         sku,
						category:    category,
						regionGroup: regionGroup,
						region:      region,
						displayName: instanceType,
						computeAttrs: domain.ComputeAttributes{
							VCPU:   vcpu,
							RAMGB:  ram,
							Family: family,
						},
					}
					filteredProducts[sku] = meta

				} else if isStorageProduct(prod, prod.Attributes) {
					category, err := catalogmap.MapAWSProduct(serviceCode)
					if err != nil {
						return nil, fmt.Errorf("aws normalize sku %s: %w", sku, err)
					}

					location := prod.Attributes["location"]
					if location == "" {
						location = prod.Attributes["regionCode"]
					}
					regionGroup, err := regionmap.MapAWSRegion(location)
					if err != nil {
						return nil, fmt.Errorf("aws normalize sku %s: %w", sku, err)
					}

					rawStorageClass := prod.Attributes["storageClass"]
					if rawStorageClass == "" {
						rawStorageClass = prod.Attributes["volumeType"]
					}
					storageClass, err := storageclassmap.MapAWSStorageClass(rawStorageClass)
					if err != nil {
						return nil, fmt.Errorf("aws normalize sku %s: %w", sku, err)
					}

					region := prod.Attributes["regionCode"]
					if region == "" {
						region = prod.Attributes["location"]
					}

					displayName := prod.Attributes["description"]
					if displayName == "" {
						displayName = fmt.Sprintf("S3 %s Storage", rawStorageClass)
					}

					meta := awsProductMeta{
						sku:         sku,
						category:    category,
						regionGroup: regionGroup,
						region:      region,
						displayName: displayName,
						storageAttrs: domain.StorageAttributes{
							SizeGB:       1,
							StorageClass: storageClass,
						},
					}
					filteredProducts[sku] = meta
				}
			}

			// Consume closing '}' of products map
			if err := consumeDelim(dec, '}'); err != nil {
				return nil, fmt.Errorf("aws normalize: products close delim: %w", err)
			}
			productsParsed = true

		case "terms":
			if !productsParsed {
				return nil, fmt.Errorf("%w: aws normalize: terms encountered before products in payload", provider.ErrPermanentFailure)
			}

			// Consume opening '{' of terms map
			if err := consumeDelim(dec, '{'); err != nil {
				return nil, fmt.Errorf("aws normalize: terms open delim: %w", err)
			}

			var hasOnDemand bool
			for dec.More() {
				termTypeToken, err := dec.Token()
				if err != nil {
					return nil, fmt.Errorf("aws normalize: read term type: %w", err)
				}
				termType := fmt.Sprintf("%v", termTypeToken)

				if termType == "OnDemand" {
					hasOnDemand = true
					if err := consumeDelim(dec, '{'); err != nil {
						return nil, fmt.Errorf("aws normalize: OnDemand open delim: %w", err)
					}

					for dec.More() {
						skuToken, err := dec.Token()
						if err != nil {
							return nil, fmt.Errorf("aws normalize: read OnDemand sku: %w", err)
						}
						sku := fmt.Sprintf("%v", skuToken)

						meta, isFiltered := filteredProducts[sku]
						if isFiltered {
							var skuTerms map[string]awsOfferTerm
							if err := dec.Decode(&skuTerms); err != nil {
								return nil, fmt.Errorf("aws normalize: decode OnDemand sku %s: %w", sku, err)
							}

							for _, term := range skuTerms {
								for _, dim := range term.PriceDimensions {
									usdStr, hasUSD := dim.PricePerUnit["USD"]
									if !hasUSD {
										continue
									}
									priceAmount, err := decimal.NewFromString(usdStr)
									if err != nil {
										return nil, fmt.Errorf("aws normalize sku %s: invalid price %q: %w", sku, usdStr, err)
									}
									observations = append(observations, buildObservation(meta, dim.Unit, priceAmount, fetchedAt))
								}
							}
						} else {
							// Discard non-matching terms without memory allocation
							if err := skipValue(dec); err != nil {
								return nil, fmt.Errorf("aws normalize: skip sku terms for %s: %w", sku, err)
							}
						}
					}

					if err := consumeDelim(dec, '}'); err != nil {
						return nil, fmt.Errorf("aws normalize: OnDemand close delim: %w", err)
					}
				} else {
					// Discard non-OnDemand terms (e.g. Reserved) without memory allocation
					if err := skipValue(dec); err != nil {
						return nil, fmt.Errorf("aws normalize: skip term %s: %w", termType, err)
					}
				}
			}

			if err := consumeDelim(dec, '}'); err != nil {
				return nil, fmt.Errorf("aws normalize: terms close delim: %w", err)
			}

			if !hasOnDemand {
				return nil, fmt.Errorf("aws normalize: missing OnDemand terms")
			}

		default:
			// Discard unknown top-level section without memory allocation
			if err := skipValue(dec); err != nil {
				return nil, fmt.Errorf("aws normalize: skip top-level key %s: %w", key, err)
			}
		}
	}

	return observations, nil
}

func skipValue(dec *json.Decoder) error {
	t, err := dec.Token()
	if err != nil {
		return err
	}
	_, isDelim := t.(json.Delim)
	if !isDelim {
		return nil
	}

	depth := 1
	for depth > 0 {
		t, err := dec.Token()
		if err != nil {
			return err
		}
		if d, ok := t.(json.Delim); ok {
			switch d {
			case '{', '[':
				depth++
			case '}', ']':
				depth--
			}
		}
	}
	return nil
}

func buildObservation(meta awsProductMeta, unit string, price decimal.Decimal, fetchedAt time.Time) domain.PriceObservation {
	return domain.PriceObservation{
		Provider:          "aws",
		ServiceCategory:   meta.category,
		SkuID:             meta.sku,
		DisplayName:       meta.displayName,
		Region:            meta.region,
		RegionGroup:       meta.regionGroup,
		Unit:              unit,
		PriceAmount:       price,
		PriceCurrency:     "USD",
		PricingModel:      "OnDemand",
		Attributes:        meta.computeAttrs,
		StorageAttributes: meta.storageAttrs,
		FetchedAt:         fetchedAt,
	}
}

func consumeDelim(dec *json.Decoder, expected rune) error {
	t, err := dec.Token()
	if err != nil {
		return err
	}
	delim, ok := t.(json.Delim)
	if !ok || rune(delim) != expected {
		return fmt.Errorf("expected %q, got %v", expected, t)
	}
	return nil
}

func isComputeInstance(product awsProduct, attrs map[string]string) bool {
	if attrs == nil {
		return false
	}

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

func isStorageProduct(product awsProduct, attrs map[string]string) bool {
	if attrs == nil {
		return false
	}
	if product.ProductFamily != "Storage" {
		return false
	}
	rawClass := attrs["storageClass"]
	if rawClass == "" {
		rawClass = attrs["volumeType"]
	}
	if rawClass == "" {
		return false
	}
	usageType := attrs["usagetype"]
	if usageType != "" && !strings.Contains(usageType, "ByteHrs") && !strings.Contains(usageType, "Storage") {
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

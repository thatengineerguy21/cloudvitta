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

type computeProductMeta struct {
	sku          string
	category     string
	regionGroup  string
	region       string
	instanceType string
	vcpu         float64
	ram          float64
	family       string
}

type rawTermPrice struct {
	unit     string
	usdPrice decimal.Decimal
}

// Normalize parses an AWS Price List JSON stream and returns normalized domain observations.
// It streams tokens using json.Decoder to minimize memory overhead on large provider files.
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
	filteredProducts := make(map[string]computeProductMeta)
	pendingTerms := make(map[string][]rawTermPrice)
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

				if !isComputeInstance(prod, prod.Attributes) {
					continue
				}

				serviceCode := prod.Attributes["servicecode"]
				if serviceCode == "" {
					serviceCode = offerCode
				}
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

				meta := computeProductMeta{
					sku:          sku,
					category:     category,
					regionGroup:  regionGroup,
					region:       region,
					instanceType: instanceType,
					vcpu:         vcpu,
					ram:          ram,
					family:       family,
				}

				filteredProducts[sku] = meta

				// If terms were encountered before products
				if terms, hasTerms := pendingTerms[sku]; hasTerms {
					for _, term := range terms {
						observations = append(observations, buildObservation(meta, term.unit, term.usdPrice, fetchedAt))
					}
					delete(pendingTerms, sku)
				}
			}

			// Consume closing '}' of products map
			if err := consumeDelim(dec, '}'); err != nil {
				return nil, fmt.Errorf("aws normalize: products close delim: %w", err)
			}

		case "terms":
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
							// If products haven't been parsed yet or this is a non-compute sku
							// If products already parsed, we skip decoding terms payload completely!
							if len(filteredProducts) > 0 {
								var discard json.RawMessage
								if err := dec.Decode(&discard); err != nil {
									return nil, fmt.Errorf("aws normalize: discard sku terms: %w", err)
								}
							} else {
								// Products not parsed yet; buffer compact terms
								var skuTerms map[string]awsOfferTerm
								if err := dec.Decode(&skuTerms); err != nil {
									return nil, fmt.Errorf("aws normalize: buffer OnDemand sku %s: %w", sku, err)
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
										pendingTerms[sku] = append(pendingTerms[sku], rawTermPrice{
											unit:     dim.Unit,
											usdPrice: priceAmount,
										})
									}
								}
							}
						}
					}

					if err := consumeDelim(dec, '}'); err != nil {
						return nil, fmt.Errorf("aws normalize: OnDemand close delim: %w", err)
					}
				} else {
					var discard json.RawMessage
					if err := dec.Decode(&discard); err != nil {
						return nil, fmt.Errorf("aws normalize: discard term %s: %w", termType, err)
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
			var discard json.RawMessage
			if err := dec.Decode(&discard); err != nil {
				return nil, fmt.Errorf("aws normalize: discard top-level key %s: %w", key, err)
			}
		}
	}

	return observations, nil
}

func buildObservation(meta computeProductMeta, unit string, price decimal.Decimal, fetchedAt time.Time) domain.PriceObservation {
	return domain.PriceObservation{
		Provider:        "aws",
		ServiceCategory: meta.category,
		SkuID:           meta.sku,
		DisplayName:     meta.instanceType,
		Region:          meta.region,
		RegionGroup:     meta.regionGroup,
		Unit:            unit,
		PriceAmount:     price,
		PriceCurrency:   "USD",
		PricingModel:    "OnDemand",
		Attributes: domain.ComputeAttributes{
			VCPU:   meta.vcpu,
			RAMGB:  meta.ram,
			Family: meta.family,
		},
		FetchedAt: fetchedAt,
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

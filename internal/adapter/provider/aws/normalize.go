package aws

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"github.com/thatengineerguy21/CloudVitta/internal/adapter/provider"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/catalogmap"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/regionmap"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/storageclassmap"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/transfertypemap"
	"github.com/thatengineerguy21/CloudVitta/internal/quarantine"
)

type awsProduct struct {
	SKU           string            `json:"sku"`
	ProductFamily string            `json:"productFamily"`
	Attributes    map[string]string `json:"attributes"`
}

type awsPriceDimension struct {
	Unit         string            `json:"unit"`
	BeginRange   string            `json:"beginRange"`
	EndRange     string            `json:"endRange"`
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
	networkAttrs domain.NetworkAttributes
}

// Normalize parses an AWS Pricing Bulk JSON file stream and returns normalized domain observations.
// It streams tokens using json.Decoder with depth-aware skipping to minimize memory overhead
// on large provider files. It enforces that products precede terms, returning ErrPermanentFailure
// if the payload order violates this streaming invariant.
// Unmapped taxonomy values are recorded to the optional quarantine sink and skipped without aborting the page.
func Normalize(r io.Reader, fetchedAt time.Time, sinks ...quarantine.Sink) ([]domain.PriceObservation, error) {
	var sink quarantine.Sink
	if len(sinks) > 0 {
		sink = sinks[0]
	}

	dec := json.NewDecoder(r)

	// Advance to opening '{' of root object
	if err := consumeDelim(dec, '{'); err != nil {
		return nil, fmt.Errorf("aws normalize: stream start: %w", err)
	}

	filteredProducts := make(map[string]awsProductMeta)
	var observations []domain.PriceObservation
	var productsParsed bool
	var offerCode string

	for dec.More() {
		keyToken, err := dec.Token()
		if err != nil {
			return nil, fmt.Errorf("aws normalize: read top-level key: %w", err)
		}
		key := fmt.Sprintf("%v", keyToken)

		switch key {
		case "offerCode":
			var val string
			if err := dec.Decode(&val); err != nil {
				return nil, fmt.Errorf("aws normalize: decode offerCode: %w", err)
			}
			offerCode = val

		case "products":
			prods, err := parseProductsMap(dec, offerCode, fetchedAt, sink)
			if err != nil {
				return nil, err
			}
			filteredProducts = prods
			productsParsed = true

		case "terms":
			if !productsParsed {
				return nil, fmt.Errorf("%w: aws normalize: terms encountered before products in payload", provider.ErrPermanentFailure)
			}

			obs, err := parseTermsMap(dec, filteredProducts, fetchedAt)
			if err != nil {
				return nil, err
			}
			observations = append(observations, obs...)

		default:
			// Discard unknown top-level section without memory allocation
			if err := provider.SkipJSONValue(dec); err != nil {
				return nil, fmt.Errorf("aws normalize: skip top-level key %s: %w", key, err)
			}
		}
	}

	return observations, nil
}

func parseProductsMap(dec *json.Decoder, offerCode string, fetchedAt time.Time, sink quarantine.Sink) (map[string]awsProductMeta, error) {
	if err := consumeDelim(dec, '{'); err != nil {
		return nil, fmt.Errorf("aws normalize: products open delim: %w", err)
	}

	filteredProducts := make(map[string]awsProductMeta)
	for dec.More() {
		skuToken, err := dec.Token()
		if err != nil {
			return nil, fmt.Errorf("aws normalize: read sku key: %w", err)
		}
		sku := fmt.Sprintf("%v", skuToken)

		var prod awsProduct
		if err := dec.Decode(&prod); err != nil {
			return nil, fmt.Errorf("aws normalize: decode product %s: %w", sku, err)
		}

		meta, err := parseSingleProduct(prod, sku, offerCode, fetchedAt, sink)
		if err != nil {
			return nil, err
		}
		if meta != nil {
			filteredProducts[sku] = *meta
		}
	}

	if err := consumeDelim(dec, '}'); err != nil {
		return nil, fmt.Errorf("aws normalize: products close delim: %w", err)
	}
	return filteredProducts, nil
}

func parseSingleProduct(prod awsProduct, sku, offerCode string, fetchedAt time.Time, sink quarantine.Sink) (*awsProductMeta, error) {
	serviceCode := prod.Attributes["servicecode"]
	if serviceCode == "" {
		serviceCode = offerCode
	}

	if isComputeInstance(prod, prod.Attributes) {
		category, err := catalogmap.MapAWSProduct(serviceCode)
		if err != nil {
			if errors.Is(err, catalogmap.ErrUnmappedProduct) {
				if sink != nil {
					_ = sink.Record(context.Background(), quarantine.UnmappedItem{
						Provider:   "aws",
						Category:   "compute",
						Kind:       "product",
						RawValue:   serviceCode,
						SkuID:      sku,
						ObservedAt: fetchedAt,
					})
				}
				slog.Warn("aws normalize: skipping SKU due to unmapped product", "sku", sku, "product", serviceCode)
				return nil, nil
			}
			return nil, fmt.Errorf("aws normalize sku %s: %w", sku, err)
		}

		location := prod.Attributes["location"]
		if location == "" {
			location = prod.Attributes["regionCode"]
		}
		regionGroup, err := regionmap.MapAWSRegion(location)
		if err != nil {
			if errors.Is(err, regionmap.ErrUnmappedRegion) {
				if sink != nil {
					_ = sink.Record(context.Background(), quarantine.UnmappedItem{
						Provider:   "aws",
						Category:   category,
						Kind:       "region",
						RawValue:   location,
						SkuID:      sku,
						ObservedAt: fetchedAt,
					})
				}
				slog.Warn("aws normalize: skipping SKU due to unmapped region", "sku", sku, "region", location)
				return nil, nil
			}
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

		return &awsProductMeta{
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
		}, nil
	}

	if isStorageProduct(prod, prod.Attributes) {
		category, err := catalogmap.MapAWSProduct(serviceCode)
		if err != nil {
			if errors.Is(err, catalogmap.ErrUnmappedProduct) {
				if sink != nil {
					_ = sink.Record(context.Background(), quarantine.UnmappedItem{
						Provider:   "aws",
						Category:   "storage",
						Kind:       "product",
						RawValue:   serviceCode,
						SkuID:      sku,
						ObservedAt: fetchedAt,
					})
				}
				slog.Warn("aws normalize: skipping SKU due to unmapped product", "sku", sku, "product", serviceCode)
				return nil, nil
			}
			return nil, fmt.Errorf("aws normalize sku %s: %w", sku, err)
		}

		location := prod.Attributes["location"]
		if location == "" {
			location = prod.Attributes["regionCode"]
		}
		regionGroup, err := regionmap.MapAWSRegion(location)
		if err != nil {
			if errors.Is(err, regionmap.ErrUnmappedRegion) {
				if sink != nil {
					_ = sink.Record(context.Background(), quarantine.UnmappedItem{
						Provider:   "aws",
						Category:   category,
						Kind:       "region",
						RawValue:   location,
						SkuID:      sku,
						ObservedAt: fetchedAt,
					})
				}
				slog.Warn("aws normalize: skipping SKU due to unmapped region", "sku", sku, "region", location)
				return nil, nil
			}
			return nil, fmt.Errorf("aws normalize sku %s: %w", sku, err)
		}

		rawStorageClass := prod.Attributes["storageClass"]
		if rawStorageClass == "" {
			rawStorageClass = prod.Attributes["volumeType"]
		}
		storageClass, err := storageclassmap.MapAWSStorageClass(rawStorageClass)
		if err != nil {
			if errors.Is(err, storageclassmap.ErrUnmappedStorageClass) {
				if sink != nil {
					_ = sink.Record(context.Background(), quarantine.UnmappedItem{
						Provider:   "aws",
						Category:   category,
						Kind:       "storage_class",
						RawValue:   rawStorageClass,
						SkuID:      sku,
						ObservedAt: fetchedAt,
					})
				}
				slog.Warn("aws normalize: skipping SKU due to unmapped storage class", "sku", sku, "storage_class", rawStorageClass)
				return nil, nil
			}
			return nil, fmt.Errorf("aws normalize sku %s: %w", sku, err)
		}

		region := prod.Attributes["regionCode"]
		if region == "" {
			region = prod.Attributes["location"]
		}

		displayName := prod.Attributes["description"]

		return &awsProductMeta{
			sku:         sku,
			category:    category,
			regionGroup: regionGroup,
			region:      region,
			displayName: displayName,
			storageAttrs: domain.StorageAttributes{
				SizeGB:       1,
				StorageClass: storageClass,
			},
		}, nil
	}

	if isNetworkProduct(prod, prod.Attributes) {
		category, err := catalogmap.MapAWSProduct(serviceCode)
		if err != nil {
			if errors.Is(err, catalogmap.ErrUnmappedProduct) {
				if sink != nil {
					_ = sink.Record(context.Background(), quarantine.UnmappedItem{
						Provider:   "aws",
						Category:   "network",
						Kind:       "product",
						RawValue:   serviceCode,
						SkuID:      sku,
						ObservedAt: fetchedAt,
					})
				}
				slog.Warn("aws normalize: skipping SKU due to unmapped product", "sku", sku, "product", serviceCode)
				return nil, nil
			}
			return nil, fmt.Errorf("aws normalize sku %s: %w", sku, err)
		}

		location := prod.Attributes["fromLocation"]
		if location == "" {
			location = prod.Attributes["location"]
		}
		if location == "" {
			location = prod.Attributes["regionCode"]
		}
		regionGroup, err := regionmap.MapAWSRegion(location)
		if err != nil {
			if errors.Is(err, regionmap.ErrUnmappedRegion) {
				if sink != nil {
					_ = sink.Record(context.Background(), quarantine.UnmappedItem{
						Provider:   "aws",
						Category:   category,
						Kind:       "region",
						RawValue:   location,
						SkuID:      sku,
						ObservedAt: fetchedAt,
					})
				}
				slog.Warn("aws normalize: skipping SKU due to unmapped region", "sku", sku, "region", location)
				return nil, nil
			}
			return nil, fmt.Errorf("aws normalize sku %s: %w", sku, err)
		}

		region := prod.Attributes["regionCode"]
		if region == "" {
			region = location
		}

		displayName := prod.Attributes["description"]
		if displayName == "" {
			displayName = "AWS Data Transfer Out"
		}

		transferType, err := transfertypemap.MapAWSTransferType(displayName)
		if err != nil {
			if errors.Is(err, transfertypemap.ErrUnmappedTransferType) {
				if sink != nil {
					_ = sink.Record(context.Background(), quarantine.UnmappedItem{
						Provider:   "aws",
						Category:   category,
						Kind:       "transfer_type",
						RawValue:   displayName,
						SkuID:      sku,
						ObservedAt: fetchedAt,
					})
				}
				slog.Warn("aws normalize: skipping SKU due to unmapped transfer type", "sku", sku, "transfer_type", displayName)
				return nil, nil
			}
			return nil, fmt.Errorf("aws normalize sku %s transfer type: %w", sku, err)
		}

		return &awsProductMeta{
			sku:         sku,
			category:    category,
			regionGroup: regionGroup,
			region:      region,
			displayName: displayName,
			networkAttrs: domain.NetworkAttributes{
				EgressGB:     1,
				TransferType: transferType,
			},
		}, nil
	}

	return nil, nil
}

func parseTermsMap(dec *json.Decoder, filteredProducts map[string]awsProductMeta, fetchedAt time.Time) ([]domain.PriceObservation, error) {
	if err := consumeDelim(dec, '{'); err != nil {
		return nil, fmt.Errorf("aws normalize: terms open delim: %w", err)
	}

	var observations []domain.PriceObservation
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

					obs, err := parseSKUTerms(skuTerms, sku, meta, fetchedAt)
					if err != nil {
						return nil, err
					}
					observations = append(observations, obs...)
				} else {
					// Discard non-matching terms without memory allocation
					if err := provider.SkipJSONValue(dec); err != nil {
						return nil, fmt.Errorf("aws normalize: skip sku terms for %s: %w", sku, err)
					}
				}
			}

			if err := consumeDelim(dec, '}'); err != nil {
				return nil, fmt.Errorf("aws normalize: OnDemand close delim: %w", err)
			}
		} else {
			// Discard non-OnDemand terms (e.g. Reserved) without memory allocation
			if err := provider.SkipJSONValue(dec); err != nil {
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

	return observations, nil
}

func parseSKUTerms(skuTerms map[string]awsOfferTerm, sku string, meta awsProductMeta, fetchedAt time.Time) ([]domain.PriceObservation, error) {
	var observations []domain.PriceObservation
	for _, term := range skuTerms {
		// Tiered Pricing Detection (PRD §16.1):
		// If there are multiple price dimensions in a term or dimension starts above 0, skip the SKU entirely.
		if len(term.PriceDimensions) > 1 {
			slog.Info("skipping AWS SKU due to tiered pricing", "provider", "aws", "sku", sku, "dimensions", len(term.PriceDimensions), "reason", "tiered_pricing_not_supported_in_v1")
			continue
		}

		for _, dim := range term.PriceDimensions {
			if dim.BeginRange != "" && dim.BeginRange != "0" {
				slog.Info("skipping AWS SKU dimension due to tiered pricing", "provider", "aws", "sku", sku, "begin_range", dim.BeginRange, "reason", "tiered_pricing_not_supported_in_v1")
				continue
			}

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
	return observations, nil
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
		NetworkAttributes: meta.networkAttrs,
		FetchedAt:         fetchedAt,
	}
}

func consumeDelim(dec *json.Decoder, expected rune) error {
	t, err := dec.Token()
	if err != nil {
		return err
	}
	d, ok := t.(json.Delim)
	if !ok || rune(d) != expected {
		return fmt.Errorf("expected delimiter %q, got %v", string(expected), t)
	}
	return nil
}

func isComputeInstance(product awsProduct, attrs map[string]string) bool {
	if attrs == nil {
		return false
	}
	if product.ProductFamily != "" && product.ProductFamily != "Compute Instance" {
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
	if product.ProductFamily != "" && product.ProductFamily != "Storage" {
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

func isNetworkProduct(product awsProduct, attrs map[string]string) bool {
	if attrs == nil {
		return false
	}
	if product.ProductFamily == "Fee" || product.ProductFamily == "Storage" || product.ProductFamily == "Compute Instance" {
		return false
	}
	if product.ProductFamily == "Data Transfer" {
		return true
	}
	if attrs["servicecode"] == "AWSDataTransfer" && (strings.Contains(attrs["usagetype"], "DataTransfer-Out") || strings.Contains(attrs["usagetype"], "AWS-Out-Bytes") || attrs["fromLocation"] != "") {
		return true
	}
	return false
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

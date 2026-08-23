package aws

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/catalogmap"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/regionmap"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/serverlessarchmap"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/serverlessunitmap"
	"github.com/thatengineerguy21/CloudVitta/internal/quarantine"
)

// isServerlessProduct reports whether an AWS product represents an AWS Lambda serverless resource.
func isServerlessProduct(product awsProduct, attrs map[string]string) bool {
	if attrs == nil {
		return false
	}
	if attrs["servicecode"] == "AWSLambda" || product.ProductFamily == "Serverless" {
		return true
	}
	if strings.Contains(attrs["usagetype"], "Lambda") || strings.Contains(attrs["group"], "AWS-Lambda") {
		return true
	}
	return false
}

// normalizeServerlessProduct normalizes an AWS Lambda serverless pricing record into product metadata.
func normalizeServerlessProduct(prod awsProduct, serviceCode, sku string, fetchedAt time.Time, sink quarantine.Sink) (*awsProductMeta, error) {
	category, err := catalogmap.MapAWSProduct(serviceCode)
	if err != nil {
		if errors.Is(err, catalogmap.ErrUnmappedProduct) {
			if sink != nil {
				_ = sink.Record(context.Background(), quarantine.UnmappedItem{
					Provider:   "aws",
					Category:   "serverless",
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

	group := prod.Attributes["group"]
	usageType := prod.Attributes["usagetype"]
	desc := prod.Attributes["description"]

	// Filter out edge computing, provisioned concurrency, SnapStart, event pollers, ephemeral storage, and managed instances.
	if strings.Contains(group, "Edge") || strings.Contains(usageType, "Edge") ||
		strings.Contains(group, "Provisioned") || strings.Contains(usageType, "Provisioned") ||
		strings.Contains(group, "SnapStart") || strings.Contains(group, "Snapshot") ||
		strings.Contains(group, "Event-Poller") || strings.Contains(group, "SQS-Event-Poller") ||
		strings.Contains(group, "Storage-Duration") || strings.Contains(usageType, "Storage") ||
		strings.Contains(usageType, "Managed-Instances") {
		return nil, nil
	}

	var componentType string
	switch {
	case strings.Contains(group, "Requests") || strings.Contains(usageType, "Request") || strings.Contains(desc, "Request"):
		componentType = domain.ComponentTypeRequestFee
	case strings.Contains(group, "Duration") || strings.Contains(usageType, "GB-Second") || strings.Contains(desc, "GB-Second") || strings.Contains(desc, "Duration"):
		componentType = domain.ComponentTypeDurationFee
	default:
		return nil, nil
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

	region := prod.Attributes["regionCode"]
	if region == "" {
		region = location
	}

	archLookupKey := group
	if archLookupKey == "" {
		archLookupKey = usageType
	}
	if archLookupKey == "" {
		archLookupKey = desc
	}

	arch, err := serverlessarchmap.MapAWSArchitecture(archLookupKey)
	if err != nil && usageType != "" && usageType != archLookupKey {
		arch, err = serverlessarchmap.MapAWSArchitecture(usageType)
	}
	if err != nil {
		if errors.Is(err, serverlessarchmap.ErrUnmappedArchitecture) {
			if sink != nil {
				_ = sink.Record(context.Background(), quarantine.UnmappedItem{
					Provider:   "aws",
					Category:   category,
					Kind:       "serverless_architecture",
					RawValue:   archLookupKey,
					SkuID:      sku,
					ObservedAt: fetchedAt,
				})
			}
			slog.Warn("aws normalize: skipping SKU due to unmapped serverless architecture", "sku", sku, "arch_key", archLookupKey)
			return nil, nil
		}
		return nil, fmt.Errorf("aws normalize sku %s: %w", sku, err)
	}

	unit, err := serverlessunitmap.MapAWSUnit(archLookupKey, componentType)
	if err != nil && usageType != "" && usageType != archLookupKey {
		unit, err = serverlessunitmap.MapAWSUnit(usageType, componentType)
	}
	if err != nil {
		if errors.Is(err, serverlessunitmap.ErrUnmappedUnit) {
			if sink != nil {
				_ = sink.Record(context.Background(), quarantine.UnmappedItem{
					Provider:   "aws",
					Category:   category,
					Kind:       "serverless_unit",
					RawValue:   archLookupKey,
					SkuID:      sku,
					ObservedAt: fetchedAt,
				})
			}
			slog.Warn("aws normalize: skipping SKU due to unmapped serverless unit", "sku", sku, "unit_key", archLookupKey)
			return nil, nil
		}
		return nil, fmt.Errorf("aws normalize sku %s: %w", sku, err)
	}

	displayName := desc
	if displayName == "" {
		if componentType == domain.ComponentTypeRequestFee {
			displayName = fmt.Sprintf("AWS Lambda Invocations (%s)", arch)
		} else {
			displayName = fmt.Sprintf("AWS Lambda Compute Duration (%s)", arch)
		}
	}

	return &awsProductMeta{
		sku:         sku,
		category:    category,
		regionGroup: regionGroup,
		region:      region,
		displayName: displayName,
		serverlessAttrs: domain.ServerlessRateAttributes{
			Architecture:  arch,
			Tier:          domain.ServerlessTierConsumption,
			ComponentType: componentType,
			Unit:          unit,
		},
	}, nil
}

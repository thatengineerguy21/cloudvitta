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
	"github.com/thatengineerguy21/CloudVitta/internal/matching/kubernetestieremap"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/regionmap"
	"github.com/thatengineerguy21/CloudVitta/internal/quarantine"
)

// isKubernetesProduct reports whether an AWS product represents an Amazon EKS control plane resource.
func isKubernetesProduct(product awsProduct, attrs map[string]string) bool {
	if attrs == nil {
		return false
	}

	usageType := attrs["usagetype"]
	group := attrs["group"]
	desc := attrs["description"]

	// Exclude EKS Auto Mode management surcharges
	if strings.Contains(usageType, "EKS-Auto") || strings.Contains(group, "EKS-Auto") || strings.Contains(desc, "Auto Mode") || strings.Contains(desc, "EKS Auto") {
		return false
	}
	// Exclude Fargate compute allocations and CRD/controller items
	if strings.Contains(usageType, "Fargate") || strings.Contains(group, "Fargate") || strings.Contains(desc, "Fargate") {
		return false
	}
	if strings.Contains(usageType, "Controller") || strings.Contains(usageType, "CRD") {
		return false
	}

	if attrs["servicecode"] == "AmazonEKS" {
		return true
	}
	if strings.Contains(usageType, "AmazonEKS") || strings.Contains(group, "AmazonEKS") {
		return true
	}
	return false
}

// normalizeKubernetesProduct normalizes an AWS EKS control plane pricing record into product metadata.
func normalizeKubernetesProduct(prod awsProduct, serviceCode, sku string, fetchedAt time.Time, sink quarantine.Sink) (*awsProductMeta, error) {
	category, err := catalogmap.MapAWSProduct(serviceCode)
	if err != nil {
		if errors.Is(err, catalogmap.ErrUnmappedProduct) {
			if sink != nil {
				_ = sink.Record(context.Background(), quarantine.UnmappedItem{
					Provider:   "aws",
					Category:   "kubernetes",
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

	region := prod.Attributes["regionCode"]
	if region == "" {
		region = location
	}

	usageType := prod.Attributes["usagetype"]
	operation := prod.Attributes["operation"]

	tierLookupKey := usageType
	if tierLookupKey == "" {
		tierLookupKey = operation
	}

	tier, err := kubernetestieremap.MapAWSTier(tierLookupKey)
	if err != nil && operation != "" && operation != tierLookupKey {
		tier, err = kubernetestieremap.MapAWSTier(operation)
	}
	if err != nil {
		if errors.Is(err, kubernetestieremap.ErrUnmappedTier) {
			if sink != nil {
				_ = sink.Record(context.Background(), quarantine.UnmappedItem{
					Provider:   "aws",
					Category:   category,
					Kind:       "kubernetes_tier",
					RawValue:   tierLookupKey,
					SkuID:      sku,
					ObservedAt: fetchedAt,
				})
			}
			slog.Warn("aws normalize: skipping SKU due to unmapped kubernetes tier", "sku", sku, "usage_type", usageType, "operation", operation)
			return nil, nil
		}
		return nil, fmt.Errorf("aws normalize sku %s: %w", sku, err)
	}

	displayName := "Amazon EKS"
	switch tier {
	case kubernetestieremap.TierExtendedSupport:
		displayName = "Amazon EKS Extended Support"
	case kubernetestieremap.TierStandard:
		displayName = "Amazon EKS Cluster"
	}

	desc := prod.Attributes["description"]
	if desc != "" {
		displayName = desc
	}

	return &awsProductMeta{
		sku:         sku,
		category:    category,
		regionGroup: regionGroup,
		region:      region,
		displayName: displayName,
		kubernetesAttrs: domain.KubernetesAttributes{
			Tier: tier,
		},
	}, nil
}

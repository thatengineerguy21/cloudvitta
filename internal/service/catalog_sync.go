package service

import (
	"context"
	"log/slog"
	"strings"

	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/store"
)

// ClassifyComputeInstanceCategory determines the canonical compute category based on family and SKU.
func ClassifyComputeInstanceCategory(family, skuID string) string {
	f := strings.ToLower(family)
	sku := strings.ToLower(skuID)

	switch {
	case strings.Contains(f, "compute") || strings.HasPrefix(f, "c") || strings.HasPrefix(sku, "standard_f") || strings.HasPrefix(sku, "c2") || strings.HasPrefix(sku, "c3"):
		return "compute_optimized"
	case strings.Contains(f, "memory") || strings.HasPrefix(f, "r") || strings.HasPrefix(f, "x") || strings.HasPrefix(sku, "standard_e") || strings.HasPrefix(sku, "standard_m") || strings.HasPrefix(sku, "m2"):
		return "memory_optimized"
	case strings.Contains(f, "gpu") || strings.Contains(f, "accelerat") || strings.HasPrefix(f, "g") || strings.HasPrefix(f, "p") || strings.HasPrefix(sku, "standard_nc") || strings.HasPrefix(sku, "standard_nd") || strings.HasPrefix(sku, "a2") || strings.HasPrefix(sku, "g2"):
		return "gpu_accelerated"
	case strings.Contains(f, "storage") || strings.HasPrefix(f, "i") || strings.HasPrefix(f, "d") || strings.HasPrefix(sku, "standard_l") || strings.HasPrefix(sku, "z3"):
		return "storage_optimized"
	default:
		return "general_purpose"
	}
}

// DetectCPUArchitecture detects if an instance uses ARM or x86_64 architecture.
func DetectCPUArchitecture(family, skuID string) string {
	f := strings.ToLower(family)
	sku := strings.ToLower(skuID)

	if strings.Contains(f, "arm") || strings.Contains(sku, "arm") ||
		strings.Contains(f, "graviton") || strings.HasSuffix(f, "g") ||
		strings.Contains(sku, "t4g") || strings.Contains(sku, "c7g") || strings.Contains(sku, "m7g") ||
		strings.Contains(f, "ps_") || strings.Contains(sku, "ps_v") || strings.Contains(sku, "pld_") ||
		strings.Contains(sku, "t2a") || strings.Contains(sku, "c4a") {
		return "arm64"
	}

	return "x86_64"
}

// DetectIsBurstable detects if an instance is burstable (e.g. AWS T-family, Azure B-series).
func DetectIsBurstable(family, skuID string) bool {
	f := strings.ToLower(family)
	sku := strings.ToLower(skuID)

	return strings.HasPrefix(f, "t") || strings.HasPrefix(sku, "t2") || strings.HasPrefix(sku, "t3") ||
		strings.HasPrefix(sku, "t4") || strings.HasPrefix(sku, "standard_b") || strings.Contains(sku, "burstable")
}

// SyncComputeCatalog extracts distinct compute instances and upserts them into compute_instance_catalog.
func SyncComputeCatalog(ctx context.Context, queries store.Querier, observations []domain.PriceObservation) error {
	type key struct {
		provider string
		skuID    string
	}

	seen := make(map[key]domain.PriceObservation)
	for _, obs := range observations {
		if obs.ServiceCategory != "compute" || obs.SkuID == "" {
			continue
		}
		k := key{provider: obs.Provider, skuID: obs.SkuID}
		if _, exists := seen[k]; !exists {
			seen[k] = obs
		}
	}

	for _, obs := range seen {
		family := obs.Attributes.Family
		if family == "" {
			parts := strings.Split(obs.SkuID, ".")
			if len(parts) > 0 {
				family = parts[0]
			} else {
				family = "general_purpose"
			}
		}

		category := ClassifyComputeInstanceCategory(family, obs.SkuID)
		arch := DetectCPUArchitecture(family, obs.SkuID)
		burstable := DetectIsBurstable(family, obs.SkuID)

		displayName := obs.DisplayName
		if displayName == "" {
			displayName = obs.SkuID
		}

		firstSeen := obs.FetchedAt
		lastSeen := obs.FetchedAt

		item := domain.ComputeCatalogItem{
			Provider:        obs.Provider,
			InstanceTypeID:  obs.SkuID,
			DisplayName:     displayName,
			InstanceFamily:  family,
			Category:        category,
			VCPU:            obs.Attributes.VCPU,
			MemoryGiB:       obs.Attributes.RAMGB,
			CPUArchitecture: arch,
			GPUCount:        0,
			IsBurstable:     burstable,
			IsCurrentGen:    true,
			FirstSeenAt:     firstSeen,
			LastSeenAt:      lastSeen,
			Attributes:      obs.Attributes,
		}

		params, err := store.ToUpsertComputeCatalogItemParams(item)
		if err != nil {
			slog.WarnContext(ctx, "failed to build upsert catalog params",
				"provider", obs.Provider,
				"sku", obs.SkuID,
				"error", err,
			)
			continue
		}

		if _, err := queries.UpsertComputeCatalogItem(ctx, params); err != nil {
			slog.WarnContext(ctx, "failed to upsert compute catalog item",
				"provider", obs.Provider,
				"sku", obs.SkuID,
				"error", err,
			)
			continue
		}
	}

	return nil
}

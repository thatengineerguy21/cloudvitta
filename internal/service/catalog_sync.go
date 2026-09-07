package service

import (
	"context"
	"log/slog"
	"strings"

	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/store"
)

// DetermineCategory classifies an instance family or SKU into one of the 8 canonical compute workload families:
// general_purpose, compute_optimized, memory_optimized, storage_optimized, gpu_accelerated, hpc, network_optimized, burstable.
func DetermineCategory(provider, instanceType string) string {
	s := strings.ToLower(strings.TrimSpace(instanceType))

	// 1. HPC (AWS: Hpc, Azure: HB, HC, H, HX, GCP: H3, H4D)
	if strings.Contains(s, "hpc") || strings.HasPrefix(s, "standard_hb") || strings.HasPrefix(s, "standard_hc") ||
		strings.HasPrefix(s, "standard_hx") || strings.HasPrefix(s, "h3-") || strings.HasPrefix(s, "h4d-") {
		return "hpc"
	}

	// 2. Network Optimized (AWS: C5n, C6in, C7gn, C8gn; Azure: FX; GCP: C4N, M4N)
	if (strings.HasPrefix(s, "c") && strings.Contains(s, "n.")) || strings.Contains(s, "in.") || strings.Contains(s, "gn.") ||
		strings.HasPrefix(s, "standard_fx") || strings.HasPrefix(s, "c4n-") || strings.HasPrefix(s, "m4n-") {
		return "network_optimized"
	}

	// 3. GPU / Accelerated (AWS: G, P, Inf, Trn, DL, F; Azure: NC, ND, NV, NG; GCP: A2, A3, G2, G4)
	if strings.Contains(s, "gpu") || strings.Contains(s, "accelerat") ||
		strings.HasPrefix(s, "g4") || strings.HasPrefix(s, "g5") || strings.HasPrefix(s, "g6") ||
		strings.HasPrefix(s, "p3") || strings.HasPrefix(s, "p4") || strings.HasPrefix(s, "p5") ||
		strings.HasPrefix(s, "inf") || strings.HasPrefix(s, "trn") || strings.HasPrefix(s, "dl1") ||
		strings.HasPrefix(s, "standard_nc") || strings.HasPrefix(s, "standard_nd") || strings.HasPrefix(s, "standard_nv") || strings.HasPrefix(s, "standard_ng") ||
		strings.HasPrefix(s, "a2-") || strings.HasPrefix(s, "a3-") || strings.HasPrefix(s, "g2-") || strings.HasPrefix(s, "g4-") {
		return "gpu_accelerated"
	}

	// 4. Storage Optimized (AWS: I, D; Azure: L; GCP: Z3)
	if strings.HasPrefix(s, "i3") || strings.HasPrefix(s, "i4") || strings.HasPrefix(s, "im4") || strings.HasPrefix(s, "is4") ||
		strings.HasPrefix(s, "d2.") || strings.HasPrefix(s, "d3.") || strings.HasPrefix(s, "d3en.") ||
		strings.HasPrefix(s, "standard_l") || strings.HasPrefix(s, "z3-") || strings.Contains(s, "storage") {
		return "storage_optimized"
	}

	// 5. Memory Optimized (AWS: R, X, U; Azure: E, Eb, EC, M; GCP: M1, M2, M3, M4, X4)
	if strings.HasPrefix(s, "r") || strings.HasPrefix(s, "x1") || strings.HasPrefix(s, "x2") || strings.HasPrefix(s, "u-") ||
		strings.HasPrefix(s, "standard_e") || strings.HasPrefix(s, "standard_m") || strings.HasPrefix(s, "m1-") || strings.HasPrefix(s, "m2-") || strings.HasPrefix(s, "m3-") || strings.HasPrefix(s, "m4-") ||
		strings.HasPrefix(s, "x4-") || strings.Contains(s, "memory") {
		return "memory_optimized"
	}

	// 6. Compute Optimized (AWS: C; Azure: F; GCP: C2, C2D)
	if strings.HasPrefix(s, "c") || strings.HasPrefix(s, "standard_f") || strings.HasPrefix(s, "c2-") || strings.HasPrefix(s, "c2d-") || strings.Contains(s, "compute") {
		return "compute_optimized"
	}

	// 7. Burstable (AWS: T; Azure: B; GCP: shared-core e2-micro/small/medium)
	if strings.HasPrefix(s, "t2") || strings.HasPrefix(s, "t3") || strings.HasPrefix(s, "t4") ||
		strings.HasPrefix(s, "standard_b") || s == "e2-micro" || s == "e2-small" || s == "e2-medium" {
		return "burstable"
	}

	// 8. General Purpose (Default: AWS M, Azure D, GCP E2/N2/N4/C3/C4)
	return "general_purpose"
}

// DetectGPUType detects GPU count and model name where applicable.
func DetectGPUType(family, skuID string) (int32, *string) {
	s := strings.ToLower(skuID + " " + family)
	var count int32 = 0
	var model string

	switch {
	case strings.Contains(s, "g4dn"):
		count = 1
		model = "NVIDIA T4"
	case strings.Contains(s, "g5"):
		count = 1
		model = "NVIDIA A10G"
	case strings.Contains(s, "g6"):
		count = 1
		model = "NVIDIA L4"
	case strings.Contains(s, "p3"):
		count = 1
		model = "NVIDIA V100"
	case strings.Contains(s, "p4d"):
		count = 8
		model = "NVIDIA A100"
	case strings.Contains(s, "p5"):
		count = 8
		model = "NVIDIA H100"
	case strings.Contains(s, "standard_nc"):
		count = 1
		model = "NVIDIA Tesla"
	case strings.Contains(s, "standard_nd"):
		count = 8
		model = "NVIDIA A100"
	case strings.Contains(s, "standard_nv"):
		count = 1
		model = "NVIDIA A10"
	case strings.Contains(s, "a2-"):
		count = 1
		model = "NVIDIA A100"
	case strings.Contains(s, "a3-"):
		count = 8
		model = "NVIDIA H100"
	case strings.Contains(s, "g2-"):
		count = 1
		model = "NVIDIA L4"
	}

	if count > 0 && model != "" {
		return count, &model
	}
	return 0, nil
}

// ClassifyComputeInstanceCategory determines the canonical compute category based on family and SKU.
func ClassifyComputeInstanceCategory(family, skuID string) string {
	cat := DetermineCategory("", skuID)
	if cat == "burstable" {
		// Preserve general_purpose classification for burstable instances in 5-category summary
		return "general_purpose"
	}
	if cat != "general_purpose" {
		return cat
	}
	fCat := DetermineCategory("", family)
	if fCat == "burstable" {
		return "general_purpose"
	}
	return fCat
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
		gpuCount, gpuType := DetectGPUType(family, obs.SkuID)

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
			GPUCount:        gpuCount,
			GPUType:         gpuType,
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

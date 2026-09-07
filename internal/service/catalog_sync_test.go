package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/service"
	"github.com/thatengineerguy21/CloudVitta/internal/store"
)

func TestClassifyComputeInstanceCategory(t *testing.T) {
	tests := []struct {
		family   string
		skuID    string
		expected string
	}{
		{"c5", "c5.xlarge", "compute_optimized"},
		{"compute", "Standard_F4s_v2", "compute_optimized"},
		{"r6i", "r6i.2xlarge", "memory_optimized"},
		{"memory", "Standard_E8s_v5", "memory_optimized"},
		{"g4dn", "g4dn.xlarge", "gpu_accelerated"},
		{"gpu", "Standard_NC6s_v3", "gpu_accelerated"},
		{"i3en", "i3en.xlarge", "storage_optimized"},
		{"storage", "Standard_L8s_v2", "storage_optimized"},
		{"t3", "t3.medium", "general_purpose"},
		{"m6i", "m6i.large", "general_purpose"},
		{"Standard_D4s_v5", "Standard_D4s_v5", "general_purpose"},
	}

	for _, tt := range tests {
		got := service.ClassifyComputeInstanceCategory(tt.family, tt.skuID)
		if got != tt.expected {
			t.Errorf("ClassifyComputeInstanceCategory(%q, %q) = %q, want %q", tt.family, tt.skuID, got, tt.expected)
		}
	}
}

func TestDetermineCategory(t *testing.T) {
	tests := []struct {
		provider     string
		instanceType string
		expected     string
	}{
		// 1. General Purpose
		{"aws", "m6i.large", "general_purpose"},
		{"azure", "Standard_D4s_v5", "general_purpose"},
		{"gcp", "n2-standard-4", "general_purpose"},
		// 2. Compute Optimized
		{"aws", "c5.xlarge", "compute_optimized"},
		{"azure", "Standard_F4s_v2", "compute_optimized"},
		{"gcp", "c2-standard-8", "compute_optimized"},
		// 3. Memory Optimized
		{"aws", "r6i.2xlarge", "memory_optimized"},
		{"azure", "Standard_E8s_v5", "memory_optimized"},
		{"gcp", "m1-ultramem-40", "memory_optimized"},
		// 4. Storage Optimized
		{"aws", "i3en.xlarge", "storage_optimized"},
		{"azure", "Standard_L8s_v2", "storage_optimized"},
		{"gcp", "z3-highmem-88", "storage_optimized"},
		// 5. GPU / Accelerated
		{"aws", "g4dn.xlarge", "gpu_accelerated"},
		{"azure", "Standard_NC6s_v3", "gpu_accelerated"},
		{"gcp", "a2-highgpu-1g", "gpu_accelerated"},
		// 6. HPC
		{"aws", "hpc6a.48xlarge", "hpc"},
		{"azure", "Standard_HB120rs_v3", "hpc"},
		{"gcp", "h3-standard-88", "hpc"},
		// 7. Network Optimized
		{"aws", "c5n.18xlarge", "network_optimized"},
		{"azure", "Standard_FX4mds", "network_optimized"},
		{"gcp", "c4n-standard-16", "network_optimized"},
		// 8. Burstable
		{"aws", "t3.medium", "burstable"},
		{"azure", "Standard_B2s", "burstable"},
		{"gcp", "e2-micro", "burstable"},
	}

	for _, tt := range tests {
		got := service.DetermineCategory(tt.provider, tt.instanceType)
		if got != tt.expected {
			t.Errorf("DetermineCategory(%q, %q) = %q, want %q", tt.provider, tt.instanceType, got, tt.expected)
		}
	}
}

func TestDetectGPUType(t *testing.T) {
	tests := []struct {
		skuID         string
		expectedCount int32
		expectedModel string
	}{
		{"g4dn.xlarge", 1, "NVIDIA T4"},
		{"g5.xlarge", 1, "NVIDIA A10G"},
		{"p4d.24xlarge", 8, "NVIDIA A100"},
		{"Standard_NC6s_v3", 1, "NVIDIA Tesla"},
		{"a2-highgpu-1g", 1, "NVIDIA A100"},
		{"m6i.large", 0, ""},
	}

	for _, tt := range tests {
		count, model := service.DetectGPUType("", tt.skuID)
		if count != tt.expectedCount {
			t.Errorf("DetectGPUType(%q) count = %d, want %d", tt.skuID, count, tt.expectedCount)
		}
		if tt.expectedModel == "" && model != nil {
			t.Errorf("DetectGPUType(%q) model = %q, want nil", tt.skuID, *model)
		} else if tt.expectedModel != "" {
			if model == nil {
				t.Errorf("DetectGPUType(%q) model = nil, want %q", tt.skuID, tt.expectedModel)
			} else if *model != tt.expectedModel {
				t.Errorf("DetectGPUType(%q) model = %q, want %q", tt.skuID, *model, tt.expectedModel)
			}
		}
	}
}

func TestDetectCPUArchitecture(t *testing.T) {
	tests := []struct {
		family   string
		skuID    string
		expected string
	}{
		{"t4g", "t4g.nano", "arm64"},
		{"c7g", "c7g.xlarge", "arm64"},
		{"graviton", "r7g.2xlarge", "arm64"},
		{"Standard_Dps_v5", "Standard_D4ps_v5", "arm64"},
		{"t2a", "t2a-standard-4", "arm64"},
		{"m6i", "m6i.large", "x86_64"},
		{"Standard_D4s_v5", "Standard_D4s_v5", "x86_64"},
		{"e2-standard-4", "e2-standard-4", "x86_64"},
	}

	for _, tt := range tests {
		got := service.DetectCPUArchitecture(tt.family, tt.skuID)
		if got != tt.expected {
			t.Errorf("DetectCPUArchitecture(%q, %q) = %q, want %q", tt.family, tt.skuID, got, tt.expected)
		}
	}
}

func TestDetectIsBurstable(t *testing.T) {
	tests := []struct {
		family   string
		skuID    string
		expected bool
	}{
		{"t2", "t2.micro", true},
		{"t3", "t3.medium", true},
		{"t4g", "t4g.small", true},
		{"Standard_B", "Standard_B2s", true},
		{"m6i", "m6i.large", false},
		{"c6i", "c6i.xlarge", false},
	}

	for _, tt := range tests {
		got := service.DetectIsBurstable(tt.family, tt.skuID)
		if got != tt.expected {
			t.Errorf("DetectIsBurstable(%q, %q) = %v, want %v", tt.family, tt.skuID, got, tt.expected)
		}
	}
}

func TestSyncComputeCatalog(t *testing.T) {
	upsertedCount := 0
	mockQ := &mockCatalogQuerier{
		upsertComputeCatalogItemFunc: func(ctx context.Context, arg store.UpsertComputeCatalogItemParams) (int64, error) {
			upsertedCount++
			return int64(upsertedCount), nil
		},
	}

	now := time.Now().UTC()
	observations := []domain.PriceObservation{
		{
			Provider:        "aws",
			ServiceCategory: "compute",
			SkuID:           "m6i.xlarge",
			DisplayName:     "General Purpose m6i.xlarge",
			FetchedAt:       now,
			Attributes: domain.ComputeAttributes{
				VCPU:   4,
				RAMGB:  16,
				Family: "m6i",
			},
		},
		{
			// Duplicate SKU in different region - should deduplicate
			Provider:        "aws",
			ServiceCategory: "compute",
			SkuID:           "m6i.xlarge",
			DisplayName:     "General Purpose m6i.xlarge",
			FetchedAt:       now,
			Attributes: domain.ComputeAttributes{
				VCPU:   4,
				RAMGB:  16,
				Family: "m6i",
			},
		},
		{
			// Non-compute observation - should be skipped
			Provider:        "aws",
			ServiceCategory: "storage",
			SkuID:           "s3-standard",
			FetchedAt:       now,
		},
		{
			Provider:        "azure",
			ServiceCategory: "compute",
			SkuID:           "Standard_D4s_v5",
			DisplayName:     "Standard_D4s_v5",
			FetchedAt:       now,
			Attributes: domain.ComputeAttributes{
				VCPU:   4,
				RAMGB:  16,
				Family: "Standard_D",
			},
		},
	}

	err := service.SyncComputeCatalog(context.Background(), mockQ, observations)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have upserted exactly 2 distinct compute instances: m6i.xlarge and Standard_D4s_v5
	if upsertedCount != 2 {
		t.Errorf("expected 2 upsert calls, got %d", upsertedCount)
	}
}

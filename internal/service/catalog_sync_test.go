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

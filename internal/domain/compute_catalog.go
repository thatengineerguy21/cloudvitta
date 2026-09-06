package domain

import (
	"time"
)

// ComputeCatalogItem represents a registered virtual machine specification.
type ComputeCatalogItem struct {
	ID              int64             `json:"id"`
	Provider        string            `json:"provider"`
	InstanceTypeID  string            `json:"instance_type_id"`
	DisplayName     string            `json:"display_name"`
	InstanceFamily  string            `json:"instance_family"`
	Category        string            `json:"category"`
	VCPU            float64           `json:"vcpu"`
	MemoryGiB       float64           `json:"memory_gib"`
	CPUArchitecture string            `json:"cpu_architecture"`
	GPUCount        int32             `json:"gpu_count"`
	GPUType         *string           `json:"gpu_type,omitempty"`
	IsBurstable     bool              `json:"is_burstable"`
	IsCurrentGen    bool              `json:"is_current_gen"`
	FirstSeenAt     time.Time         `json:"first_seen_at"`
	LastSeenAt      time.Time         `json:"last_seen_at"`
	Attributes      ComputeAttributes `json:"attributes"`
}

// ComputeCatalogSummary provides inventory count rollups.
type ComputeCatalogSummary struct {
	TotalInstances    int64                       `json:"total_instances"`
	ProviderTotals    map[string]int64            `json:"provider_totals"`
	CategoryBreakdown map[string]map[string]int64 `json:"category_breakdown"`
}

// CatalogFilter defines filtering parameters for listing compute instances.
type CatalogFilter struct {
	Provider       *string
	Category       *string
	InstanceFamily *string
	MinVCPU        *float64
	MaxVCPU        *float64
	MinMemoryGiB   *float64
	MaxMemoryGiB   *float64
	Limit          int32
	Offset         int32
}

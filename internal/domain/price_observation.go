package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

// ComputeAttributes holds normalized attributes for compute resources.
type ComputeAttributes struct {
	VCPU   float64 `json:"vcpu"`
	RAMGB  float64 `json:"ram_gb"`
	Family string  `json:"family"`
}

// StorageAttributes holds normalized attributes for storage resources.
type StorageAttributes struct {
	SizeGB       float64 `json:"size_gb"`
	StorageClass string  `json:"storage_class"`
	IOPS         *int    `json:"iops,omitempty"`
}

// PriceObservation represents a single normalized cloud pricing observation.
type PriceObservation struct {
	Provider          string            `json:"provider"`
	ServiceCategory   string            `json:"service_category"`
	SkuID             string            `json:"sku_id"`
	DisplayName       string            `json:"display_name"`
	Region            string            `json:"region"`
	RegionGroup       string            `json:"region_group"`
	Unit              string            `json:"unit"`
	PriceAmount       decimal.Decimal   `json:"price_amount"`
	PriceCurrency     string            `json:"price_currency"`
	PricingModel      string            `json:"pricing_model"`
	Attributes        ComputeAttributes `json:"attributes,omitempty"`
	StorageAttributes StorageAttributes `json:"storage_attributes,omitempty"`
	FetchedAt         time.Time         `json:"fetched_at"`
}

// FetchResult bundles a batch of observations and the storage path of their raw payload.
type FetchResult struct {
	Observations []PriceObservation
	RawGCSPath   string
}

// ScoredComputeObservation represents a scored and filtered compute observation.
type ScoredComputeObservation struct {
	Observation       PriceObservation
	MatchQuality      string
	MatchDeltaPct     float64
	MissingAttributes []string
}

// ScoredStorageObservation represents a scored and filtered storage observation.
type ScoredStorageObservation struct {
	Observation       PriceObservation
	MatchQuality      string
	MatchDeltaPct     float64
	MissingAttributes []string
	MonthlyCost       decimal.Decimal
}

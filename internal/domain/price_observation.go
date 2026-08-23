package domain

import (
	"encoding/json"
	"fmt"
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
	// IOPS holds provisioned IOPS where reported (e.g. block storage EBS gp3/io2).
	// Currently unpopulated for S3/Blob/GCS object storage in stage 1.4.
	IOPS *int `json:"iops,omitempty"`
}

// NetworkAttributes holds normalized attributes for networking / data transfer resources.
type NetworkAttributes struct {
	EgressGB     float64 `json:"egress_gb"`
	TransferType string  `json:"transfer_type,omitempty"`
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
	Attributes              ComputeAttributes       `json:"attributes,omitempty"`
	StorageAttributes       StorageAttributes       `json:"storage_attributes,omitempty"`
	NetworkAttributes       NetworkAttributes       `json:"network_attributes,omitempty"`
	DatabaseRDBMSAttributes DatabaseRDBMSAttributes `json:"database_rdbms_attributes,omitempty"`
	FetchedAt               time.Time               `json:"fetched_at"`
}

// FetchResult bundles a batch of observations, raw storage path, and unmapped count.
type FetchResult struct {
	Observations  []PriceObservation
	RawGCSPath    string
	UnmappedCount int
}

// ScoredComputeObservation represents a scored and filtered compute observation.
type ScoredComputeObservation struct {
	Observation       PriceObservation
	MatchQuality      string
	MatchDeltaPct     float64
	MissingAttributes []string
}

// MarshalAttributes serializes the category-specific attributes of an observation to JSON bytes.
func MarshalAttributes(obs PriceObservation) ([]byte, error) {
	switch obs.ServiceCategory {
	case "storage":
		return json.Marshal(obs.StorageAttributes)
	case "network":
		return json.Marshal(obs.NetworkAttributes)
	case "database_rdbms":
		return json.Marshal(obs.DatabaseRDBMSAttributes)
	default:
		return json.Marshal(obs.Attributes)
	}
}

// UnmarshalAttributes deserializes raw JSON bytes into the corresponding category attribute struct.
func UnmarshalAttributes(serviceCategory string, raw []byte) (ComputeAttributes, StorageAttributes, NetworkAttributes, DatabaseRDBMSAttributes, error) {
	var computeAttrs ComputeAttributes
	var storageAttrs StorageAttributes
	var networkAttrs NetworkAttributes
	var databaseAttrs DatabaseRDBMSAttributes

	if len(raw) == 0 {
		return computeAttrs, storageAttrs, networkAttrs, databaseAttrs, nil
	}

	switch serviceCategory {
	case "storage":
		if err := json.Unmarshal(raw, &storageAttrs); err != nil {
			return computeAttrs, storageAttrs, networkAttrs, databaseAttrs, fmt.Errorf("unmarshal storage attributes: %w", err)
		}
	case "network":
		if err := json.Unmarshal(raw, &networkAttrs); err != nil {
			return computeAttrs, storageAttrs, networkAttrs, databaseAttrs, fmt.Errorf("unmarshal network attributes: %w", err)
		}
	case "database_rdbms":
		if err := json.Unmarshal(raw, &databaseAttrs); err != nil {
			return computeAttrs, storageAttrs, networkAttrs, databaseAttrs, fmt.Errorf("unmarshal database attributes: %w", err)
		}
	default:
		if err := json.Unmarshal(raw, &computeAttrs); err != nil {
			return computeAttrs, storageAttrs, networkAttrs, databaseAttrs, fmt.Errorf("unmarshal compute attributes: %w", err)
		}
	}

	return computeAttrs, storageAttrs, networkAttrs, databaseAttrs, nil
}

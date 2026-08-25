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
	Provider                 string                   `json:"provider"`
	ServiceCategory          string                   `json:"service_category"`
	SkuID                    string                   `json:"sku_id"`
	DisplayName              string                   `json:"display_name"`
	Region                   string                   `json:"region"`
	RegionGroup              string                   `json:"region_group"`
	Unit                     string                   `json:"unit"`
	PriceAmount              decimal.Decimal          `json:"price_amount"`
	PriceCurrency            string                   `json:"price_currency"`
	PricingModel             string                   `json:"pricing_model"`
	Attributes               ComputeAttributes        `json:"attributes,omitempty"`
	StorageAttributes        StorageAttributes        `json:"storage_attributes,omitempty"`
	NetworkAttributes        NetworkAttributes        `json:"network_attributes,omitempty"`
	DatabaseRDBMSAttributes  DatabaseRDBMSAttributes  `json:"database_rdbms_attributes,omitempty"`
	DatabaseNoSQLAttributes  DatabaseNoSQLAttributes  `json:"database_nosql_attributes,omitempty"`
	KubernetesAttributes     KubernetesAttributes     `json:"kubernetes_attributes,omitempty"`
	ServerlessRateAttributes ServerlessRateAttributes `json:"serverless_attributes,omitempty"`
	FetchedAt                time.Time                `json:"fetched_at"`
}

// ItemClassification represents the 3-way triage classification of an ingested item.
type ItemClassification string

const (
	ItemClassificationNormalized  ItemClassification = "normalized"
	ItemClassificationQuarantined ItemClassification = "quarantined"
	ItemClassificationIgnored     ItemClassification = "ignored"
)

// NormalizationResult captures normalized observations alongside ignored item count.
type NormalizationResult struct {
	Observations []PriceObservation
	IgnoredCount int
}

// FetchResult bundles a batch of observations, raw storage path, unmapped count, and ignored count.
type FetchResult struct {
	Observations  []PriceObservation
	RawGCSPath    string
	UnmappedCount int
	IgnoredCount  int
}

// ScoredComputeObservation represents a scored and filtered compute observation.
type ScoredComputeObservation struct {
	Observation       PriceObservation
	MatchQuality      string
	MatchDeltaPct     float64
	MissingAttributes []string
}

// AttributeMarshaler serializes domain attributes of a PriceObservation into JSON bytes.
type AttributeMarshaler func(obs PriceObservation) ([]byte, error)

// AttributeUnmarshaler deserializes JSON bytes into the domain attributes of a PriceObservation.
type AttributeUnmarshaler func(obs *PriceObservation, raw []byte) error

type categoryAttributeCodec struct {
	marshal   AttributeMarshaler
	unmarshal AttributeUnmarshaler
}

var categoryCodecs = map[string]categoryAttributeCodec{}

// RegisterCategoryAttributeCodec registers a marshal/unmarshal codec for a service category.
func RegisterCategoryAttributeCodec(category string, marshal AttributeMarshaler, unmarshal AttributeUnmarshaler) {
	categoryCodecs[category] = categoryAttributeCodec{
		marshal:   marshal,
		unmarshal: unmarshal,
	}
}

func init() {
	RegisterCategoryAttributeCodec("compute",
		func(obs PriceObservation) ([]byte, error) {
			return json.Marshal(obs.Attributes)
		},
		func(obs *PriceObservation, raw []byte) error {
			if len(raw) == 0 {
				return nil
			}
			return json.Unmarshal(raw, &obs.Attributes)
		},
	)
	RegisterCategoryAttributeCodec("storage",
		func(obs PriceObservation) ([]byte, error) {
			return json.Marshal(obs.StorageAttributes)
		},
		func(obs *PriceObservation, raw []byte) error {
			if len(raw) == 0 {
				return nil
			}
			return json.Unmarshal(raw, &obs.StorageAttributes)
		},
	)
	RegisterCategoryAttributeCodec("network",
		func(obs PriceObservation) ([]byte, error) {
			return json.Marshal(obs.NetworkAttributes)
		},
		func(obs *PriceObservation, raw []byte) error {
			if len(raw) == 0 {
				return nil
			}
			return json.Unmarshal(raw, &obs.NetworkAttributes)
		},
	)
	RegisterCategoryAttributeCodec("database_rdbms",
		func(obs PriceObservation) ([]byte, error) {
			return json.Marshal(obs.DatabaseRDBMSAttributes)
		},
		func(obs *PriceObservation, raw []byte) error {
			if len(raw) == 0 {
				return nil
			}
			return json.Unmarshal(raw, &obs.DatabaseRDBMSAttributes)
		},
	)
	RegisterCategoryAttributeCodec("database_nosql",
		func(obs PriceObservation) ([]byte, error) {
			return json.Marshal(obs.DatabaseNoSQLAttributes)
		},
		func(obs *PriceObservation, raw []byte) error {
			if len(raw) == 0 {
				return nil
			}
			return json.Unmarshal(raw, &obs.DatabaseNoSQLAttributes)
		},
	)
	RegisterCategoryAttributeCodec("kubernetes",
		func(obs PriceObservation) ([]byte, error) {
			return json.Marshal(obs.KubernetesAttributes)
		},
		func(obs *PriceObservation, raw []byte) error {
			if len(raw) == 0 {
				return nil
			}
			return json.Unmarshal(raw, &obs.KubernetesAttributes)
		},
	)
	RegisterCategoryAttributeCodec("serverless",
		func(obs PriceObservation) ([]byte, error) {
			return json.Marshal(obs.ServerlessRateAttributes)
		},
		func(obs *PriceObservation, raw []byte) error {
			if len(raw) == 0 {
				return nil
			}
			return json.Unmarshal(raw, &obs.ServerlessRateAttributes)
		},
	)
}

// MarshalAttributes serializes the category-specific attributes of an observation to JSON bytes.
func MarshalAttributes(obs PriceObservation) ([]byte, error) {
	codec, ok := categoryCodecs[obs.ServiceCategory]
	if !ok {
		codec = categoryCodecs["compute"]
	}
	return codec.marshal(obs)
}

// UnmarshalAttributes deserializes raw JSON bytes into the corresponding category attribute fields of obs.
func UnmarshalAttributes(obs *PriceObservation, raw []byte) error {
	if len(raw) == 0 {
		return nil
	}
	codec, ok := categoryCodecs[obs.ServiceCategory]
	if !ok {
		codec = categoryCodecs["compute"]
	}
	if err := codec.unmarshal(obs, raw); err != nil {
		return fmt.Errorf("unmarshal %s attributes: %w", obs.ServiceCategory, err)
	}
	return nil
}

package domain

// DatabaseNoSQLAttributes holds normalized attributes for managed NoSQL database resources.
type DatabaseNoSQLAttributes struct {
	DataModel        string  `json:"data_model"`                  // "document", "key_value", "wide_column", "graph", "multi_model"
	PricingMode      string  `json:"pricing_mode"`                // "provisioned", "on_demand", "serverless"
	ReadUnits        float64 `json:"read_units"`                  // Provisioned reads/sec (RCU/RU/s) or request rate
	WriteUnits       float64 `json:"write_units"`                 // Provisioned writes/sec (WCU/RU/s) or request rate
	StorageGB        float64 `json:"storage_gb"`                  // Storage capacity in GiB
	StorageClass     string  `json:"storage_class,omitempty"`     // "standard", "infrequent_access", "analytical"
	MultiRegion      bool    `json:"multi_region"`                // Multi-region replication / global distribution
	ReplicationZones int     `json:"replication_zones,omitempty"` // Number of replica regions (default: 1)
	ComponentType    string  `json:"component_type"`              // "throughput", "storage", "request_operations"
}

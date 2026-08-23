package domain

// DatabaseRDBMSAttributes holds normalized attributes for relational database resources.
type DatabaseRDBMSAttributes struct {
	Engine         string  `json:"engine"`                   // "postgresql", "mysql", "sqlserver", "mariadb", "oracle"
	VCPU           float64 `json:"vcpu"`                     // Number of vCPUs (0 for pure storage rows)
	RAMGB          float64 `json:"ram_gb"`                   // Memory capacity in GiB (0 for pure storage rows)
	StorageGB      float64 `json:"storage_gb"`               // Storage capacity in GiB (0 for pure instance rows)
	IOPS           *int    `json:"iops,omitempty"`           // Provisioned IOPS
	MultiAZ        bool    `json:"multi_az"`                 // High Availability deployment
	DeploymentTier string  `json:"deployment_tier"`          // "standard", "flexible", "burstable", "aurora"
	StorageFamily  string  `json:"storage_family,omitempty"` // "gp3", "gp2", "io1", "ssd", "hdd"
	ComponentType  string  `json:"component_type"`           // "instance" or "storage"
}

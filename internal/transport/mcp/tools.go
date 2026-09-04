package mcp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shopspring/decimal"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/databaseenginemap"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/kubernetestieremap"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/nosqldatamodelmap"
	"github.com/thatengineerguy21/CloudVitta/internal/service"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// maxAllowedStorageSizeGB represents the upper bound on single-request storage size (1 PB).
var maxAllowedStorageSizeGB = decimal.NewFromInt(1_000_000)

// maxAllowedNetworkEgressGB represents the upper bound on single-request egress size (10 PB).
var maxAllowedNetworkEgressGB = decimal.NewFromInt(10_000_000)

// Bounds for database and NoSQL parameters.
const (
	maxAllowedDatabaseStorageGB = 1_000_000.0
	maxAllowedNoSQLStorageGB    = 1_000_000.0
	maxAllowedNoSQLThroughput   = 10_000_000.0
)

// --- Shared Output Types ---

// PriceDetail represents price details in a comparison result entry.
type PriceDetail struct {
	Amount   decimal.Decimal `json:"amount"`
	Unit     string          `json:"unit"`
	Currency string          `json:"currency"`
}

// ProviderWarning represents an item in the warnings array.
type ProviderWarning struct {
	Provider string `json:"provider"`
	Code     string `json:"code"`
	Message  string `json:"message"`
}

// ResponseMeta represents standard metadata in comparison and calculation response envelopes.
type ResponseMeta struct {
	APIVersion  string    `json:"api_version"`
	GeneratedAt time.Time `json:"generated_at"`
	Query       any       `json:"query"`
}

// ComputeQueryMeta represents strongly-typed query parameters in compute comparison responses.
type ComputeQueryMeta struct {
	Category     string   `json:"category"`
	Region       string   `json:"region"`
	Currency     string   `json:"currency"`
	VCPU         *float64 `json:"vcpu,omitempty"`
	RAMGB        *float64 `json:"ram_gb,omitempty"`
	Family       string   `json:"family,omitempty"`
	StrictFamily *bool    `json:"strict_family,omitempty"`
}

// StorageQueryMeta represents strongly-typed query parameters in storage comparison responses.
type StorageQueryMeta struct {
	Category     string           `json:"category"`
	Region       string           `json:"region"`
	Currency     string           `json:"currency"`
	SizeGB       *decimal.Decimal `json:"size_gb,omitempty"`
	StorageClass string           `json:"storage_class,omitempty"`
}

// NetworkQueryMeta represents strongly-typed query parameters in network comparison responses.
type NetworkQueryMeta struct {
	Category     string           `json:"category"`
	Region       string           `json:"region"`
	Currency     string           `json:"currency"`
	EgressGB     *decimal.Decimal `json:"egress_gb,omitempty"`
	TransferType string           `json:"transfer_type,omitempty"`
}

// ComputeResultEntry represents a single provider compute result item.
type ComputeResultEntry struct {
	Provider            string                   `json:"provider"`
	SkuID               string                   `json:"sku_id"`
	MatchedSpec         domain.ComputeAttributes `json:"matched_spec"`
	MatchQuality        string                   `json:"match_quality"`
	MatchDeltaPct       float64                  `json:"match_delta_pct"`
	MissingAttributes   []string                 `json:"missing_attributes"`
	Price               PriceDetail              `json:"price"`
	NormalizedHourlyUSD decimal.Decimal          `json:"normalized_hourly_usd"`
	FetchedAt           time.Time                `json:"fetched_at"`
	Stale               bool                     `json:"stale"`
}

// ComputeComparisonResponse represents the full compute comparison response envelope.
type ComputeComparisonResponse struct {
	Meta     ResponseMeta         `json:"meta"`
	Results  []ComputeResultEntry `json:"results"`
	Warnings []ProviderWarning    `json:"warnings"`
}

// StorageResultEntry represents a single provider storage result item.
type StorageResultEntry struct {
	Provider          string                   `json:"provider"`
	SkuID             string                   `json:"sku_id"`
	MatchedSpec       domain.StorageAttributes `json:"matched_spec"`
	MatchQuality      string                   `json:"match_quality"`
	MatchDeltaPct     float64                  `json:"match_delta_pct"`
	MissingAttributes []string                 `json:"missing_attributes"`
	Price             PriceDetail              `json:"price"`
	MonthlyCostUSD    decimal.Decimal          `json:"monthly_cost_usd"`
	FetchedAt         time.Time                `json:"fetched_at"`
	Stale             bool                     `json:"stale"`
}

// StorageComparisonResponse represents the full storage comparison response envelope.
type StorageComparisonResponse struct {
	Meta     ResponseMeta         `json:"meta"`
	Results  []StorageResultEntry `json:"results"`
	Warnings []ProviderWarning    `json:"warnings"`
}

// NetworkResultEntry represents a single provider network result item.
type NetworkResultEntry struct {
	Provider          string                   `json:"provider"`
	SkuID             string                   `json:"sku_id"`
	MatchedSpec       domain.NetworkAttributes `json:"matched_spec"`
	MatchQuality      string                   `json:"match_quality"`
	MatchDeltaPct     float64                  `json:"match_delta_pct"`
	MissingAttributes []string                 `json:"missing_attributes"`
	Price             PriceDetail              `json:"price"`
	MonthlyCostUSD    decimal.Decimal          `json:"monthly_cost_usd"`
	FetchedAt         time.Time                `json:"fetched_at"`
	Stale             bool                     `json:"stale"`
}

// NetworkComparisonResponse represents the full network comparison response envelope.
type NetworkComparisonResponse struct {
	Meta     ResponseMeta         `json:"meta"`
	Results  []NetworkResultEntry `json:"results"`
	Warnings []ProviderWarning    `json:"warnings"`
}

// DatabaseQueryMeta represents strongly-typed query parameters in database comparison responses.
type DatabaseQueryMeta struct {
	Category      string   `json:"category"`
	Region        string   `json:"region"`
	Currency      string   `json:"currency"`
	Engine        string   `json:"engine,omitempty"`
	VCPU          *float64 `json:"vcpu,omitempty"`
	RAMGB         *float64 `json:"ram_gb,omitempty"`
	StorageGB     *float64 `json:"storage_gb,omitempty"`
	IOPS          *int     `json:"iops,omitempty"`
	MultiAZ       *bool    `json:"multi_az,omitempty"`
	StorageFamily string   `json:"storage_family,omitempty"`
}

// DatabaseResultEntry represents a single provider database result item.
type DatabaseResultEntry struct {
	Provider            string                         `json:"provider"`
	SkuID               string                         `json:"sku_id"`
	MatchedSpec         domain.DatabaseRDBMSAttributes `json:"matched_spec"`
	MatchQuality        string                         `json:"match_quality"`
	MatchDeltaPct       float64                        `json:"match_delta_pct"`
	MissingAttributes   []string                       `json:"missing_attributes"`
	Price               PriceDetail                    `json:"price"`
	NormalizedHourlyUSD decimal.Decimal                `json:"normalized_hourly_usd"`
	FetchedAt           time.Time                      `json:"fetched_at"`
	Stale               bool                           `json:"stale"`
}

// DatabaseComparisonResponse represents the full database comparison response envelope.
type DatabaseComparisonResponse struct {
	Meta     ResponseMeta          `json:"meta"`
	Results  []DatabaseResultEntry `json:"results"`
	Warnings []ProviderWarning     `json:"warnings"`
}

// DatabaseNoSQLQueryMeta represents strongly-typed query parameters in NoSQL database comparison responses.
type DatabaseNoSQLQueryMeta struct {
	Category     string   `json:"category"`
	Region       string   `json:"region"`
	Currency     string   `json:"currency"`
	DataModel    string   `json:"data_model,omitempty"`
	PricingMode  string   `json:"pricing_mode,omitempty"`
	ReadUnits    *float64 `json:"read_units,omitempty"`
	WriteUnits   *float64 `json:"write_units,omitempty"`
	StorageGB    *float64 `json:"storage_gb,omitempty"`
	StorageClass string   `json:"storage_class,omitempty"`
	MultiRegion  *bool    `json:"multi_region,omitempty"`
}

// DatabaseNoSQLResultEntry represents a single provider NoSQL database result item.
type DatabaseNoSQLResultEntry struct {
	Provider            string                         `json:"provider"`
	SkuID               string                         `json:"sku_id"`
	MatchedSpec         domain.DatabaseNoSQLAttributes `json:"matched_spec"`
	MatchQuality        string                         `json:"match_quality"`
	MatchDeltaPct       float64                        `json:"match_delta_pct"`
	MissingAttributes   []string                       `json:"missing_attributes"`
	Price               PriceDetail                    `json:"price"`
	NormalizedHourlyUSD decimal.Decimal                `json:"normalized_hourly_usd"`
	FetchedAt           time.Time                      `json:"fetched_at"`
	Stale               bool                           `json:"stale"`
}

// DatabaseNoSQLComparisonResponse represents the full NoSQL database comparison response envelope.
type DatabaseNoSQLComparisonResponse struct {
	Meta     ResponseMeta               `json:"meta"`
	Results  []DatabaseNoSQLResultEntry `json:"results"`
	Warnings []ProviderWarning          `json:"warnings"`
}

// KubernetesQueryMeta represents strongly-typed query parameters in kubernetes comparison responses.
type KubernetesQueryMeta struct {
	Category        string `json:"category"`
	Region          string `json:"region"`
	Currency        string `json:"currency"`
	Tier            string `json:"tier,omitempty"`
	ClusterTopology string `json:"cluster_topology,omitempty"`
}

// KubernetesResultEntry represents a single provider kubernetes result item.
type KubernetesResultEntry struct {
	Provider            string                      `json:"provider"`
	SkuID               string                      `json:"sku_id"`
	MatchedSpec         domain.KubernetesAttributes `json:"matched_spec"`
	MatchQuality        string                      `json:"match_quality"`
	MatchDeltaPct       float64                     `json:"match_delta_pct"`
	MissingAttributes   []string                    `json:"missing_attributes"`
	Price               PriceDetail                 `json:"price"`
	NormalizedHourlyUSD decimal.Decimal             `json:"normalized_hourly_usd"`
	FetchedAt           time.Time                   `json:"fetched_at"`
	Stale               bool                        `json:"stale"`
}

// KubernetesComparisonResponse represents the full kubernetes comparison response envelope.
type KubernetesComparisonResponse struct {
	Meta     ResponseMeta            `json:"meta"`
	Results  []KubernetesResultEntry `json:"results"`
	Warnings []ProviderWarning       `json:"warnings"`
}

// ServerlessResultEntry represents a single provider result item for serverless comparison.
type ServerlessResultEntry struct {
	Provider             string                          `json:"provider"`
	SkuID                string                          `json:"sku_id"`
	MatchedSpec          domain.ServerlessRateAttributes `json:"matched_spec"`
	MatchQuality         string                          `json:"match_quality"`
	MatchDeltaPct        float64                         `json:"match_delta_pct"`
	MissingAttributes    []string                        `json:"missing_attributes"`
	Price                PriceDetail                     `json:"price"`
	NormalizedHourlyUSD  decimal.Decimal                 `json:"normalized_hourly_usd"`
	NormalizedMonthlyUSD decimal.Decimal                 `json:"normalized_monthly_usd"`
	FetchedAt            time.Time                       `json:"fetched_at"`
	Stale                bool                            `json:"stale"`
}

// ServerlessQueryMeta represents the query parameters echoed back in serverless comparison metadata.
type ServerlessQueryMeta struct {
	Category            string   `json:"category"`
	Region              string   `json:"region"`
	Currency            string   `json:"currency"`
	Architecture        string   `json:"architecture"`
	Tier                string   `json:"tier"`
	RequestsPerMonth    *float64 `json:"requests_per_month,omitempty"`
	MemoryMB            *float64 `json:"memory_mb,omitempty"`
	ExecutionDurationMS *float64 `json:"execution_duration_ms,omitempty"`
}

// ServerlessComparisonResponse represents the full serverless comparison response envelope.
type ServerlessComparisonResponse struct {
	Meta     ResponseMeta            `json:"meta"`
	Results  []ServerlessResultEntry `json:"results"`
	Warnings []ProviderWarning       `json:"warnings"`
}

// CalculateCategoryResult represents a single category result inside a provider.
type CalculateCategoryResult struct {
	SkuID               string          `json:"sku_id"`
	MatchQuality        string          `json:"match_quality"`
	MatchDeltaPct       float64         `json:"match_delta_pct"`
	MissingAttributes   []string        `json:"missing_attributes"`
	Stale               bool            `json:"stale"`
	NormalizedHourlyUSD decimal.Decimal `json:"normalized_hourly_usd"`
}

// CalculateProviderResult represents a provider's overall calculation result.
type CalculateProviderResult struct {
	Provider                        string                             `json:"provider"`
	Categories                      map[string]CalculateCategoryResult `json:"categories"`
	TotalNormalizedHourlyUSD        *decimal.Decimal                   `json:"total_normalized_hourly_usd,omitempty"`
	PartialTotalNormalizedHourlyUSD *decimal.Decimal                   `json:"partial_total_normalized_hourly_usd,omitempty"`
	Partial                         bool                               `json:"partial"`
}

// CalculateResponse represents the full calculate response envelope.
type CalculateResponse struct {
	Meta     ResponseMeta              `json:"meta"`
	Results  []CalculateProviderResult `json:"results"`
	Warnings []ProviderWarning         `json:"warnings"`
}

// --- Tool Input Definitions ---

// CompareComputeInput defines parameters for compare_compute tool.
type CompareComputeInput struct {
	VCPU         *float64 `json:"vcpu,omitempty" jsonschema:"Requested number of virtual CPUs (e.g. 2, 4, 8)"`
	RAMGB        *float64 `json:"ram_gb,omitempty" jsonschema:"Requested RAM in gigabytes (e.g. 8, 16, 32)"`
	Family       string   `json:"family,omitempty" jsonschema:"Preferred instance family filter (e.g. general_purpose, compute_optimized, memory_optimized, t3, c5)"`
	Region       string   `json:"region,omitempty" jsonschema:"Canonical region group (e.g. us-east, us-west, eu-west, ap-southeast)"`
	Currency     string   `json:"currency,omitempty" jsonschema:"Target currency code (default: USD)"`
	StrictFamily *bool    `json:"strict_family,omitempty" jsonschema:"Strict instance family matching (default: true)"`
}

// CompareStorageInput defines parameters for compare_storage tool.
type CompareStorageInput struct {
	SizeGB       *float64 `json:"size_gb,omitempty" jsonschema:"Requested storage size in GB (e.g. 500, max 1000000)"`
	StorageClass string   `json:"storage_class,omitempty" jsonschema:"Requested canonical storage class (e.g. standard, infrequent_access, archive)"`
	Region       string   `json:"region,omitempty" jsonschema:"Canonical region group (default: us-east)"`
	Currency     string   `json:"currency,omitempty" jsonschema:"Target currency code (default: USD)"`
}

// CompareNetworkInput defines parameters for compare_network tool.
type CompareNetworkInput struct {
	EgressGB     *float64 `json:"egress_gb,omitempty" jsonschema:"Requested network egress in GB (e.g. 500, max 10000000)"`
	TransferType string   `json:"transfer_type,omitempty" jsonschema:"Canonical transfer type (intra_region, inter_region, internet_egress)"`
	Region       string   `json:"region,omitempty" jsonschema:"Canonical region group (default: us-east)"`
	Currency     string   `json:"currency,omitempty" jsonschema:"Target currency code (default: USD)"`
}

// CompareDatabaseInput defines parameters for compare_database tool.
type CompareDatabaseInput struct {
	Engine        string   `json:"engine,omitempty" jsonschema:"Requested database engine (postgresql, mysql, sqlserver)"`
	VCPU          *float64 `json:"vcpu,omitempty" jsonschema:"Requested vCPU count (e.g. 2, 4, 8)"`
	RAMGB         *float64 `json:"ram_gb,omitempty" jsonschema:"Requested RAM in gigabytes (e.g. 8, 16, 32)"`
	StorageGB     *float64 `json:"storage_gb,omitempty" jsonschema:"Requested database storage size in GB (max 1000000)"`
	IOPS          *int     `json:"iops,omitempty" jsonschema:"Requested provisioned IOPS (e.g. 3000)"`
	MultiAZ       *bool    `json:"multi_az,omitempty" jsonschema:"High Availability / Multi-AZ deployment (default: false)"`
	StorageFamily string   `json:"storage_family,omitempty" jsonschema:"Storage family preference (e.g. gp3, ssd, io1)"`
	Region        string   `json:"region,omitempty" jsonschema:"Canonical region group (default: us-east)"`
	Currency      string   `json:"currency,omitempty" jsonschema:"Target currency code (default: USD)"`
}

// CompareDatabaseNoSQLInput defines parameters for compare_database_nosql tool.
type CompareDatabaseNoSQLInput struct {
	DataModel    string   `json:"data_model,omitempty" jsonschema:"Requested NoSQL data model (document, key_value, wide_column, graph, multi_model)"`
	PricingMode  string   `json:"pricing_mode,omitempty" jsonschema:"Requested pricing mode (provisioned, on_demand, serverless; default: provisioned)"`
	ReadUnits    *float64 `json:"read_units,omitempty" jsonschema:"Requested reads per second (RCU/RU/s, max 10000000)"`
	WriteUnits   *float64 `json:"write_units,omitempty" jsonschema:"Requested writes per second (WCU/RU/s, max 10000000)"`
	StorageGB    *float64 `json:"storage_gb,omitempty" jsonschema:"Requested storage in GB (max 1000000)"`
	StorageClass string   `json:"storage_class,omitempty" jsonschema:"Storage class preference (standard, infrequent_access, analytical)"`
	MultiRegion  *bool    `json:"multi_region,omitempty" jsonschema:"High Availability / Multi-Region replication (default: false)"`
	Region       string   `json:"region,omitempty" jsonschema:"Canonical region group (default: us-east)"`
	Currency     string   `json:"currency,omitempty" jsonschema:"Target currency code (default: USD)"`
}

// CompareKubernetesInput defines parameters for compare_kubernetes tool.
type CompareKubernetesInput struct {
	Tier            string `json:"tier,omitempty" jsonschema:"Requested Kubernetes control plane tier (free, standard, extended_support)"`
	ClusterTopology string `json:"cluster_topology,omitempty" jsonschema:"GCP cluster topology (zonal, regional, autopilot)"`
	Region          string `json:"region,omitempty" jsonschema:"Canonical region group (default: us-east)"`
	Currency        string `json:"currency,omitempty" jsonschema:"Target currency code (default: USD)"`
}

// CompareServerlessInput defines parameters for compare_serverless tool.
type CompareServerlessInput struct {
	Architecture        string   `json:"architecture,omitempty" jsonschema:"Requested CPU architecture (x86_64, arm64; default: x86_64)"`
	Tier                string   `json:"tier,omitempty" jsonschema:"Requested serverless tier (consumption, flex_consumption, 1st_gen, 2nd_gen; default: consumption)"`
	RequestsPerMonth    *float64 `json:"requests_per_month,omitempty" jsonschema:"Monthly invocation requests (default: 1000000)"`
	MemoryMB            *float64 `json:"memory_mb,omitempty" jsonschema:"Function allocated memory in MB (128 - 10240, default: 512)"`
	ExecutionDurationMS *float64 `json:"execution_duration_ms,omitempty" jsonschema:"Average execution duration in ms (1 - 900000, default: 200)"`
	Region              string   `json:"region,omitempty" jsonschema:"Canonical region group (default: us-east)"`
	Currency            string   `json:"currency,omitempty" jsonschema:"Target currency code (default: USD)"`
}

// ComputeRequirements defines compute requirements for calculate_workload.
type ComputeRequirements struct {
	VCPU   float64 `json:"vcpu,omitempty" jsonschema:"Requested vCPU count (e.g. 2, 4, 8)"`
	RAMGB  float64 `json:"ram_gb,omitempty" jsonschema:"Requested RAM in gigabytes (e.g. 8, 16, 32)"`
	Family string  `json:"family,omitempty" jsonschema:"Preferred instance family (e.g. general_purpose, compute_optimized)"`
}

// StorageRequirements defines storage requirements for calculate_workload.
type StorageRequirements struct {
	SizeGB       float64 `json:"size_gb,omitempty" jsonschema:"Requested storage capacity in GB (e.g. 100)"`
	StorageClass string  `json:"storage_class,omitempty" jsonschema:"Requested storage class (e.g. standard, infrequent_access, archive)"`
}

// NetworkRequirements defines network requirements for calculate_workload.
type NetworkRequirements struct {
	EgressGB     float64 `json:"egress_gb,omitempty" jsonschema:"Requested network egress in GB (e.g. 50)"`
	TransferType string  `json:"transfer_type,omitempty" jsonschema:"Requested transfer type (e.g. internet_egress, intra_region)"`
}

// DatabaseRDBMSRequirements defines relational database requirements for calculate_workload.
type DatabaseRDBMSRequirements struct {
	Engine        string  `json:"engine,omitempty" jsonschema:"Requested database engine (e.g. postgresql, mysql, sqlserver)"`
	VCPU          float64 `json:"vcpu,omitempty" jsonschema:"Requested vCPU count (e.g. 2, 4, 8)"`
	RAMGB         float64 `json:"ram_gb,omitempty" jsonschema:"Requested RAM in gigabytes (e.g. 8, 16, 32)"`
	StorageGB     float64 `json:"storage_gb,omitempty" jsonschema:"Requested database storage capacity in GB (e.g. 100)"`
	IOPS          *int    `json:"iops,omitempty" jsonschema:"Requested provisioned IOPS (e.g. 3000)"`
	MultiAZ       bool    `json:"multi_az,omitempty" jsonschema:"High Availability / Multi-AZ deployment (default: false)"`
	StorageFamily string  `json:"storage_family,omitempty" jsonschema:"Storage family preference (e.g. gp3, ssd)"`
}

// DatabaseRequirements is an alias for DatabaseRDBMSRequirements.
type DatabaseRequirements = DatabaseRDBMSRequirements

// DatabaseNoSQLRequirements defines NoSQL database requirements for calculate_workload.
type DatabaseNoSQLRequirements struct {
	DataModel    string  `json:"data_model,omitempty" jsonschema:"Requested NoSQL data model (e.g. document, key_value)"`
	PricingMode  string  `json:"pricing_mode,omitempty" jsonschema:"Requested pricing mode (provisioned, on_demand, serverless; default: provisioned)"`
	ReadUnits    float64 `json:"read_units,omitempty" jsonschema:"Requested reads per second (e.g. 100)"`
	WriteUnits   float64 `json:"write_units,omitempty" jsonschema:"Requested writes per second (e.g. 50)"`
	StorageGB    float64 `json:"storage_gb,omitempty" jsonschema:"Requested storage in GB (e.g. 50)"`
	StorageClass string  `json:"storage_class,omitempty" jsonschema:"Storage class preference (e.g. standard)"`
	MultiRegion  bool    `json:"multi_region,omitempty" jsonschema:"Multi-Region replication requested (default: false)"`
}

// KubernetesRequirements defines Kubernetes control plane requirements for calculate_workload.
type KubernetesRequirements struct {
	Tier            string `json:"tier,omitempty" jsonschema:"Requested Kubernetes control plane tier (free, standard, extended_support)"`
	ClusterTopology string `json:"cluster_topology,omitempty" jsonschema:"GCP cluster topology (zonal, regional, autopilot)"`
}

// ServerlessRequirements defines serverless compute requirements for calculate_workload.
type ServerlessRequirements struct {
	Architecture        string   `json:"architecture,omitempty" jsonschema:"Requested CPU architecture (x86_64, arm64; default: x86_64)"`
	Tier                string   `json:"tier,omitempty" jsonschema:"Requested serverless tier (consumption, flex_consumption, 1st_gen, 2nd_gen; default: consumption)"`
	RequestsPerMonth    *float64 `json:"requests_per_month,omitempty" jsonschema:"Monthly invocation requests (default: 1000000)"`
	MemoryMB            *float64 `json:"memory_mb,omitempty" jsonschema:"Function allocated memory in MB (128 - 10240, default: 512)"`
	ExecutionDurationMS *float64 `json:"execution_duration_ms,omitempty" jsonschema:"Average execution duration in ms (1 - 900000, default: 200)"`
}

// CalculateWorkloadInput defines parameters for calculate_workload tool.
type CalculateWorkloadInput struct {
	Region        string                     `json:"region,omitempty" jsonschema:"Canonical region group (default: us-east)"`
	Currency      string                     `json:"currency,omitempty" jsonschema:"Target currency code (default: USD)"`
	StrictFamily  *bool                      `json:"strict_family,omitempty" jsonschema:"Strict instance family matching (default: true)"`
	Compute       *ComputeRequirements       `json:"compute,omitempty" jsonschema:"Compute requirements (vcpu, ram_gb, family)"`
	Storage       *StorageRequirements       `json:"storage,omitempty" jsonschema:"Storage requirements (size_gb, storage_class)"`
	Network       *NetworkRequirements       `json:"network,omitempty" jsonschema:"Network requirements (egress_gb, transfer_type)"`
	DatabaseRDBMS *DatabaseRDBMSRequirements `json:"database_rdbms,omitempty" jsonschema:"Relational database requirements (engine, vcpu, ram_gb, storage_gb, iops, multi_az, storage_family)"`
	Database      *DatabaseRDBMSRequirements `json:"database,omitempty" jsonschema:"Alias for database_rdbms"`
	DatabaseNoSQL *DatabaseNoSQLRequirements `json:"database_nosql,omitempty" jsonschema:"NoSQL database requirements (data_model, pricing_mode, read_units, write_units, storage_gb, storage_class, multi_region)"`
	Kubernetes    *KubernetesRequirements    `json:"kubernetes,omitempty" jsonschema:"Kubernetes requirements (tier, cluster_topology)"`
	Serverless    *ServerlessRequirements    `json:"serverless,omitempty" jsonschema:"Serverless requirements (architecture, tier, requests_per_month, memory_mb, execution_duration_ms)"`
}

// GetProviderStatusInput defines parameters for get_provider_status tool.
type GetProviderStatusInput struct {
	Provider string `json:"provider" jsonschema:"Cloud provider identifier (e.g. aws, azure, gcp, oracle, ibm, alibaba, digitalocean)"`
}

// validateComputeRequirements validates compute requirements for calculate_workload.
func validateComputeRequirements(req ComputeRequirements) (*domain.ComputeAttributes, error) {
	if req.VCPU <= 0 {
		return nil, fmt.Errorf("%w: compute.vcpu must be a positive number", service.ErrInvalidParameters)
	}
	if req.RAMGB <= 0 {
		return nil, fmt.Errorf("%w: compute.ram_gb must be a positive number", service.ErrInvalidParameters)
	}
	return &domain.ComputeAttributes{
		VCPU:   req.VCPU,
		RAMGB:  req.RAMGB,
		Family: req.Family,
	}, nil
}

// validateStorageRequirements validates storage requirements for calculate_workload.
func validateStorageRequirements(req StorageRequirements) (*domain.StorageAttributes, error) {
	if req.SizeGB <= 0 {
		return nil, fmt.Errorf("%w: storage.size_gb must be a positive number", service.ErrInvalidParameters)
	}
	maxStorageF, _ := maxAllowedStorageSizeGB.Float64()
	if req.SizeGB > maxStorageF {
		return nil, fmt.Errorf("%w: storage.size_gb exceeds maximum limit of %s GB (1 PB)", service.ErrInvalidParameters, maxAllowedStorageSizeGB.String())
	}
	return &domain.StorageAttributes{
		SizeGB:       req.SizeGB,
		StorageClass: req.StorageClass,
	}, nil
}

// validateNetworkRequirements validates network requirements for calculate_workload.
func validateNetworkRequirements(req NetworkRequirements) (*domain.NetworkAttributes, error) {
	if req.EgressGB < 0 {
		return nil, fmt.Errorf("%w: network.egress_gb cannot be negative", service.ErrInvalidParameters)
	}
	maxNetworkF, _ := maxAllowedNetworkEgressGB.Float64()
	if req.EgressGB > maxNetworkF {
		return nil, fmt.Errorf("%w: network.egress_gb exceeds maximum limit of %s GB (10 PB)", service.ErrInvalidParameters, maxAllowedNetworkEgressGB.String())
	}
	return &domain.NetworkAttributes{
		EgressGB:     req.EgressGB,
		TransferType: req.TransferType,
	}, nil
}

// validateDatabaseRDBMSParams validates database parameters and resolves canonical engine.
func validateDatabaseRDBMSParams(engine string, vcpu, ramgb, storageGB float64, iops *int, multiAZ bool, storageFamily, prefix string) (*domain.DatabaseRDBMSAttributes, error) {
	rawEngine := strings.TrimSpace(engine)
	var canonicalEngine string
	if rawEngine != "" {
		canonical, err := databaseenginemap.ResolveCanonicalEngine(rawEngine)
		if err != nil || !databaseenginemap.IsSupportedStageEngine(canonical) {
			param := "engine"
			if prefix != "" {
				param = prefix + ".engine"
			}
			return nil, fmt.Errorf("%w: %s must be a supported database engine (postgresql, mysql, sqlserver)", service.ErrInvalidParameters, param)
		}
		canonicalEngine = canonical
	}

	if vcpu <= 0 {
		param := "vcpu"
		if prefix != "" {
			param = prefix + ".vcpu"
		}
		return nil, fmt.Errorf("%w: %s must be a positive number", service.ErrInvalidParameters, param)
	}
	if ramgb <= 0 {
		param := "ram_gb"
		if prefix != "" {
			param = prefix + ".ram_gb"
		}
		return nil, fmt.Errorf("%w: %s must be a positive number", service.ErrInvalidParameters, param)
	}
	if storageGB <= 0 {
		param := "storage_gb"
		if prefix != "" {
			param = prefix + ".storage_gb"
		}
		return nil, fmt.Errorf("%w: %s must be a positive number", service.ErrInvalidParameters, param)
	}
	if storageGB > maxAllowedDatabaseStorageGB {
		param := "storage_gb"
		if prefix != "" {
			param = prefix + ".storage_gb"
		}
		if prefix != "" {
			return nil, fmt.Errorf("%w: %s exceeds maximum limit of 1,000,000 GB (1 PB)", service.ErrInvalidParameters, param)
		}
		return nil, fmt.Errorf("%w: %s must be a positive number up to 1000000", service.ErrInvalidParameters, param)
	}
	if iops != nil && *iops <= 0 {
		param := "iops"
		if prefix != "" {
			param = prefix + ".iops"
		}
		return nil, fmt.Errorf("%w: %s must be a positive integer", service.ErrInvalidParameters, param)
	}

	return &domain.DatabaseRDBMSAttributes{
		Engine:        canonicalEngine,
		VCPU:          vcpu,
		RAMGB:         ramgb,
		StorageGB:     storageGB,
		IOPS:          iops,
		MultiAZ:       multiAZ,
		StorageFamily: storageFamily,
	}, nil
}

// validateDatabaseRDBMSRequirements validates DatabaseRDBMSRequirements for calculate_workload.
func validateDatabaseRDBMSRequirements(req DatabaseRDBMSRequirements) (*domain.DatabaseRDBMSAttributes, error) {
	return validateDatabaseRDBMSParams(req.Engine, req.VCPU, req.RAMGB, req.StorageGB, req.IOPS, req.MultiAZ, req.StorageFamily, "database_rdbms")
}

// validateDatabaseNoSQLParams validates NoSQL database parameters and resolves canonical data model and pricing mode.
func validateDatabaseNoSQLParams(dataModel, pricingMode string, readUnits, writeUnits, storageGB float64, storageClass string, multiRegion bool, prefix string) (*domain.DatabaseNoSQLAttributes, error) {
	rawModel := strings.TrimSpace(dataModel)
	var canonicalModel string
	if rawModel != "" {
		canonical, err := nosqldatamodelmap.ResolveCanonicalDataModel(rawModel)
		if err != nil || !nosqldatamodelmap.IsSupportedStageDataModel(canonical) {
			param := "data_model"
			if prefix != "" {
				param = prefix + ".data_model"
			}
			return nil, fmt.Errorf("%w: %s must be a supported NoSQL data model (document, key_value, wide_column, graph, multi_model)", service.ErrInvalidParameters, param)
		}
		canonicalModel = canonical
	}

	rawPricingMode := strings.ToLower(strings.TrimSpace(pricingMode))
	if rawPricingMode == "" {
		rawPricingMode = "provisioned"
	}
	if rawPricingMode != "provisioned" && rawPricingMode != "on_demand" && rawPricingMode != "serverless" {
		param := "pricing_mode"
		if prefix != "" {
			param = prefix + ".pricing_mode"
		}
		return nil, fmt.Errorf("%w: %s must be provisioned, on_demand, or serverless", service.ErrInvalidParameters, param)
	}

	if readUnits < 0 || readUnits > maxAllowedNoSQLThroughput {
		param := "read_units"
		if prefix != "" {
			param = prefix + ".read_units"
		}
		return nil, fmt.Errorf("%w: %s must be a non-negative number up to 10000000", service.ErrInvalidParameters, param)
	}
	if writeUnits < 0 || writeUnits > maxAllowedNoSQLThroughput {
		param := "write_units"
		if prefix != "" {
			param = prefix + ".write_units"
		}
		return nil, fmt.Errorf("%w: %s must be a non-negative number up to 10000000", service.ErrInvalidParameters, param)
	}
	if storageGB < 0 || storageGB > maxAllowedNoSQLStorageGB {
		param := "storage_gb"
		if prefix != "" {
			param = prefix + ".storage_gb"
		}
		return nil, fmt.Errorf("%w: %s must be a non-negative number up to 1000000", service.ErrInvalidParameters, param)
	}

	return &domain.DatabaseNoSQLAttributes{
		DataModel:    canonicalModel,
		PricingMode:  rawPricingMode,
		ReadUnits:    readUnits,
		WriteUnits:   writeUnits,
		StorageGB:    storageGB,
		StorageClass: storageClass,
		MultiRegion:  multiRegion,
	}, nil
}

// validateDatabaseNoSQLRequirements validates DatabaseNoSQLRequirements for calculate_workload.
func validateDatabaseNoSQLRequirements(req DatabaseNoSQLRequirements) (*domain.DatabaseNoSQLAttributes, error) {
	return validateDatabaseNoSQLParams(req.DataModel, req.PricingMode, req.ReadUnits, req.WriteUnits, req.StorageGB, req.StorageClass, req.MultiRegion, "database_nosql")
}

// validateKubernetesParams validates Kubernetes control plane parameters and resolves canonical tier.
func validateKubernetesParams(tier, clusterTopology, prefix string) (*domain.KubernetesAttributes, error) {
	rawTier := strings.TrimSpace(tier)
	canonicalTier := kubernetestieremap.TierStandard
	if rawTier != "" {
		resolved, err := kubernetestieremap.ResolveCanonicalTier(rawTier)
		if err != nil || !kubernetestieremap.IsSupportedStageTier(resolved) {
			param := "tier"
			if prefix != "" {
				param = prefix + ".tier"
			}
			return nil, fmt.Errorf("%w: %s must be a supported Kubernetes tier (free, standard, extended_support)", service.ErrInvalidParameters, param)
		}
		canonicalTier = resolved
	}

	rawTopology := strings.ToLower(strings.TrimSpace(clusterTopology))
	var topo domain.ClusterTopology
	if rawTopology != "" {
		switch domain.ClusterTopology(rawTopology) {
		case domain.ClusterTopologyZonal, domain.ClusterTopologyRegional, domain.ClusterTopologyAutopilot:
			topo = domain.ClusterTopology(rawTopology)
		default:
			param := "cluster_topology"
			if prefix != "" {
				param = prefix + ".cluster_topology"
			}
			return nil, fmt.Errorf("%w: %s must be one of: zonal, regional, autopilot", service.ErrInvalidParameters, param)
		}
	}

	return &domain.KubernetesAttributes{
		Tier:            canonicalTier,
		ClusterTopology: topo,
	}, nil
}

// validateKubernetesRequirements validates KubernetesRequirements for calculate_workload.
func validateKubernetesRequirements(req KubernetesRequirements) (*domain.KubernetesAttributes, error) {
	return validateKubernetesParams(req.Tier, req.ClusterTopology, "kubernetes")
}

// validateServerlessRequirements validates serverless compute requirements.
func validateServerlessRequirements(req ServerlessRequirements) (*service.ServerlessWorkload, error) {
	var rawReqs, rawMem, rawDur string
	if req.RequestsPerMonth != nil {
		rawReqs = strconv.FormatFloat(*req.RequestsPerMonth, 'f', -1, 64)
	}
	if req.MemoryMB != nil {
		rawMem = strconv.FormatFloat(*req.MemoryMB, 'f', -1, 64)
	}
	if req.ExecutionDurationMS != nil {
		rawDur = strconv.FormatFloat(*req.ExecutionDurationMS, 'f', -1, 64)
	}
	wl, err := service.ParseRawServerlessWorkload(service.RawServerlessParams{
		Architecture:        req.Architecture,
		Tier:                req.Tier,
		RequestsPerMonth:    rawReqs,
		MemoryMB:            rawMem,
		ExecutionDurationMS: rawDur,
	})
	if err != nil {
		return nil, err
	}
	return &wl, nil
}

// defaultUningestedWarnings returns static warnings for providers scheduled for stage 4.
func defaultUningestedWarnings() []ProviderWarning {
	return []ProviderWarning{
		{Provider: "oracle", Code: "not_yet_ingested", Message: "Oracle OCI ingestion lands in stage 4."},
		{Provider: "ibm", Code: "not_yet_ingested", Message: "IBM Cloud ingestion lands in stage 4."},
		{Provider: "alibaba", Code: "not_yet_ingested", Message: "Alibaba Cloud ingestion lands in stage 4."},
		{Provider: "digitalocean", Code: "not_yet_ingested", Message: "DigitalOcean ingestion lands in stage 4."},
	}
}

func defaultStage3Warnings() []ProviderWarning {
	return defaultUningestedWarnings()
}

// instrumentTool wraps a tool handler with OpenTelemetry tracing spans, metrics recording, and structured slog logging.
func instrumentTool[T any](toolName string, cfg *serverConfig, handler sdk.ToolHandlerFor[T, any]) sdk.ToolHandlerFor[T, any] {
	var toolCounter metric.Int64Counter
	var toolDuration metric.Float64Histogram
	if cfg != nil && cfg.meter != nil {
		toolCounter, _ = cfg.meter.Int64Counter("mcp.tool.calls", metric.WithDescription("Total invocations of MCP tools"))
		toolDuration, _ = cfg.meter.Float64Histogram("mcp.tool.duration_ms", metric.WithDescription("Duration of MCP tool executions in milliseconds"))
	}

	return func(ctx context.Context, req *sdk.CallToolRequest, input T) (*sdk.CallToolResult, any, error) {
		start := time.Now()
		var span trace.Span
		if cfg != nil && cfg.tracer != nil {
			ctx, span = cfg.tracer.Start(ctx, "mcp.tool."+toolName, trace.WithAttributes(attribute.String("mcp.tool", toolName)))
			defer span.End()
		}

		slog.DebugContext(ctx, "executing MCP tool", "tool", toolName)

		callRes, out, err := handler(ctx, req, input)
		durationMs := float64(time.Since(start).Microseconds()) / 1000.0

		status := "success"
		if err != nil {
			status = "error"
			if span != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
			}
			slog.WarnContext(ctx, "MCP tool execution completed with error", "tool", toolName, "error", err, "duration_ms", durationMs)
		} else {
			slog.DebugContext(ctx, "MCP tool execution completed successfully", "tool", toolName, "duration_ms", durationMs)
		}

		if toolCounter != nil {
			toolCounter.Add(ctx, 1, metric.WithAttributes(attribute.String("tool", toolName), attribute.String("status", status)))
		}
		if toolDuration != nil {
			toolDuration.Record(ctx, durationMs, metric.WithAttributes(attribute.String("tool", toolName), attribute.String("status", status)))
		}

		return callRes, out, err
	}
}

// handleCompareCompute creates the tool handler for compare_compute.
func handleCompareCompute(pricingSvc *service.PricingService) sdk.ToolHandlerFor[CompareComputeInput, any] {
	return func(ctx context.Context, req *sdk.CallToolRequest, input CompareComputeInput) (*sdk.CallToolResult, any, error) {
		if pricingSvc == nil {
			return nil, nil, errors.New("pricing service unavailable")
		}

		region := input.Region
		if region == "" {
			region = "us-east"
		}

		warnings := defaultStage3Warnings()

		currency := input.Currency
		reqCurrency := currency
		if reqCurrency == "" {
			reqCurrency = "USD"
		}
		if currency != "" && currency != "USD" {
			warnings = append(warnings, ProviderWarning{
				Provider: "system",
				Code:     "non_usd_currency_unsupported",
				Message:  "Currency conversion is not yet supported. Prices are returned in USD.",
			})
		}
		currency = "USD"

		strictFamily := true
		if input.StrictFamily != nil {
			strictFamily = *input.StrictFamily
		}

		var reqVCPU float64
		if input.VCPU != nil {
			if *input.VCPU <= 0 {
				return nil, nil, MapServiceError(fmt.Errorf("%w: vcpu must be a positive number", service.ErrInvalidParameters))
			}
			reqVCPU = *input.VCPU
		}

		var reqRAMGB float64
		if input.RAMGB != nil {
			if *input.RAMGB <= 0 {
				return nil, nil, MapServiceError(fmt.Errorf("%w: ram_gb must be a positive number", service.ErrInvalidParameters))
			}
			reqRAMGB = *input.RAMGB
		}

		target := service.MatchTarget{
			VCPU:         reqVCPU,
			RAMGB:        reqRAMGB,
			Family:       input.Family,
			StrictFamily: strictFamily,
			Category:     "compute",
		}

		compRes, err := pricingSvc.Compare(ctx, "compute", region, target)
		if err != nil {
			return nil, nil, MapServiceError(err)
		}

		for _, w := range compRes.Warnings {
			warnings = append(warnings, ProviderWarning{
				Provider: w.Provider,
				Code:     w.Code,
				Message:  w.Message,
			})
		}

		var results []ComputeResultEntry
		for _, item := range compRes.Results {
			results = append(results, ComputeResultEntry{
				Provider:          item.Provider,
				SkuID:             item.SkuID,
				MatchedSpec:       item.MatchedCompute,
				MatchQuality:      item.MatchQuality,
				MatchDeltaPct:     item.MatchDeltaPct,
				MissingAttributes: item.MissingAttributes,
				Price: PriceDetail{
					Amount:   item.PriceAmount,
					Unit:     item.Unit,
					Currency: currency,
				},
				NormalizedHourlyUSD: item.HourlyCost,
				FetchedAt:           item.FetchedAt,
				Stale:               item.Stale,
			})
		}

		queryMeta := ComputeQueryMeta{
			Category:     "compute",
			Region:       region,
			Currency:     reqCurrency,
			Family:       input.Family,
			StrictFamily: input.StrictFamily,
		}
		if reqVCPU > 0 {
			queryMeta.VCPU = &reqVCPU
		}
		if reqRAMGB > 0 {
			queryMeta.RAMGB = &reqRAMGB
		}

		resp := &ComputeComparisonResponse{
			Meta: ResponseMeta{
				APIVersion:  "v1",
				GeneratedAt: time.Now().UTC(),
				Query:       queryMeta,
			},
			Results:  results,
			Warnings: warnings,
		}

		return nil, resp, nil
	}
}

// handleCompareStorage creates the tool handler for compare_storage.
func handleCompareStorage(pricingSvc *service.PricingService) sdk.ToolHandlerFor[CompareStorageInput, any] {
	return func(ctx context.Context, req *sdk.CallToolRequest, input CompareStorageInput) (*sdk.CallToolResult, any, error) {
		if pricingSvc == nil {
			return nil, nil, errors.New("pricing service unavailable")
		}

		region := input.Region
		if region == "" {
			region = "us-east"
		}

		warnings := defaultStage3Warnings()

		currency := input.Currency
		reqCurrency := currency
		if reqCurrency == "" {
			reqCurrency = "USD"
		}
		if currency != "" && currency != "USD" {
			warnings = append(warnings, ProviderWarning{
				Provider: "system",
				Code:     "non_usd_currency_unsupported",
				Message:  "Currency conversion is not yet supported. Prices are returned in USD.",
			})
		}
		currency = "USD"

		sizeGB := decimal.NewFromInt(1)
		var explicitSize *decimal.Decimal
		if input.SizeGB != nil {
			parsed := decimal.NewFromFloat(*input.SizeGB)
			if parsed.LessThanOrEqual(decimal.Zero) || parsed.GreaterThan(maxAllowedStorageSizeGB) {
				return nil, nil, MapServiceError(fmt.Errorf("%w: size_gb must be a positive number no greater than %s", service.ErrInvalidParameters, maxAllowedStorageSizeGB.String()))
			}
			sizeGB = parsed
			explicitSize = &sizeGB
		}

		sizeF, _ := sizeGB.Float64()
		target := service.MatchTarget{
			SizeGB:       sizeF,
			StorageClass: input.StorageClass,
			Category:     "storage",
		}

		compRes, err := pricingSvc.Compare(ctx, "storage", region, target)
		if err != nil {
			return nil, nil, MapServiceError(err)
		}

		for _, w := range compRes.Warnings {
			warnings = append(warnings, ProviderWarning{
				Provider: w.Provider,
				Code:     w.Code,
				Message:  w.Message,
			})
		}

		var results []StorageResultEntry
		for _, item := range compRes.Results {
			results = append(results, StorageResultEntry{
				Provider:          item.Provider,
				SkuID:             item.SkuID,
				MatchedSpec:       item.MatchedStorage,
				MatchQuality:      item.MatchQuality,
				MatchDeltaPct:     item.MatchDeltaPct,
				MissingAttributes: item.MissingAttributes,
				Price: PriceDetail{
					Amount:   item.PriceAmount,
					Unit:     item.Unit,
					Currency: currency,
				},
				MonthlyCostUSD: item.MonthlyCost,
				FetchedAt:      item.FetchedAt,
				Stale:          item.Stale,
			})
		}

		queryMeta := StorageQueryMeta{
			Category:     "storage",
			Region:       region,
			Currency:     reqCurrency,
			SizeGB:       explicitSize,
			StorageClass: input.StorageClass,
		}

		resp := &StorageComparisonResponse{
			Meta: ResponseMeta{
				APIVersion:  "v1",
				GeneratedAt: time.Now().UTC(),
				Query:       queryMeta,
			},
			Results:  results,
			Warnings: warnings,
		}

		return nil, resp, nil
	}
}

// handleCompareNetwork creates the tool handler for compare_network.
func handleCompareNetwork(pricingSvc *service.PricingService) sdk.ToolHandlerFor[CompareNetworkInput, any] {
	return func(ctx context.Context, req *sdk.CallToolRequest, input CompareNetworkInput) (*sdk.CallToolResult, any, error) {
		if pricingSvc == nil {
			return nil, nil, errors.New("pricing service unavailable")
		}

		region := input.Region
		if region == "" {
			region = "us-east"
		}

		warnings := defaultStage3Warnings()

		currency := input.Currency
		reqCurrency := currency
		if reqCurrency == "" {
			reqCurrency = "USD"
		}
		if currency != "" && currency != "USD" {
			warnings = append(warnings, ProviderWarning{
				Provider: "system",
				Code:     "non_usd_currency_unsupported",
				Message:  "Currency conversion is not yet supported. Prices are returned in USD.",
			})
		}
		currency = "USD"

		egressGB := decimal.NewFromInt(1)
		var explicitEgress *decimal.Decimal
		if input.EgressGB != nil {
			parsed := decimal.NewFromFloat(*input.EgressGB)
			if parsed.LessThanOrEqual(decimal.Zero) || parsed.GreaterThan(maxAllowedNetworkEgressGB) {
				return nil, nil, MapServiceError(fmt.Errorf("%w: egress_gb must be a positive number no greater than %s", service.ErrInvalidParameters, maxAllowedNetworkEgressGB.String()))
			}
			egressGB = parsed
			explicitEgress = &egressGB
		}

		egressF, _ := egressGB.Float64()
		target := service.MatchTarget{
			EgressGB:     egressF,
			TransferType: input.TransferType,
			Category:     "network",
		}

		compRes, err := pricingSvc.Compare(ctx, "network", region, target)
		if err != nil {
			return nil, nil, MapServiceError(err)
		}

		for _, w := range compRes.Warnings {
			warnings = append(warnings, ProviderWarning{
				Provider: w.Provider,
				Code:     w.Code,
				Message:  w.Message,
			})
		}

		var results []NetworkResultEntry
		for _, item := range compRes.Results {
			results = append(results, NetworkResultEntry{
				Provider:          item.Provider,
				SkuID:             item.SkuID,
				MatchedSpec:       item.MatchedNetwork,
				MatchQuality:      item.MatchQuality,
				MatchDeltaPct:     item.MatchDeltaPct,
				MissingAttributes: item.MissingAttributes,
				Price: PriceDetail{
					Amount:   item.PriceAmount,
					Unit:     item.Unit,
					Currency: currency,
				},
				MonthlyCostUSD: item.MonthlyCost,
				FetchedAt:      item.FetchedAt,
				Stale:          item.Stale,
			})
		}

		queryMeta := NetworkQueryMeta{
			Category:     "network",
			Region:       region,
			Currency:     reqCurrency,
			EgressGB:     explicitEgress,
			TransferType: input.TransferType,
		}

		resp := &NetworkComparisonResponse{
			Meta: ResponseMeta{
				APIVersion:  "v1",
				GeneratedAt: time.Now().UTC(),
				Query:       queryMeta,
			},
			Results:  results,
			Warnings: warnings,
		}

		return nil, resp, nil
	}
}

// handleCompareDatabase creates the tool handler for compare_database.
func handleCompareDatabase(pricingSvc *service.PricingService) sdk.ToolHandlerFor[CompareDatabaseInput, any] {
	return func(ctx context.Context, req *sdk.CallToolRequest, input CompareDatabaseInput) (*sdk.CallToolResult, any, error) {
		if pricingSvc == nil {
			return nil, nil, errors.New("pricing service unavailable")
		}

		region := input.Region
		if region == "" {
			region = "us-east"
		}

		warnings := defaultStage3Warnings()

		currency := input.Currency
		reqCurrency := currency
		if reqCurrency == "" {
			reqCurrency = "USD"
		}
		if currency != "" && currency != "USD" {
			warnings = append(warnings, ProviderWarning{
				Provider: "system",
				Code:     "non_usd_currency_unsupported",
				Message:  "Currency conversion is not yet supported. Prices are returned in USD.",
			})
		}
		currency = "USD"

		rawEngine := strings.TrimSpace(input.Engine)
		var canonicalEngine string
		if rawEngine != "" {
			canonical, err := databaseenginemap.ResolveCanonicalEngine(rawEngine)
			if err != nil || !databaseenginemap.IsSupportedStageEngine(canonical) {
				return nil, nil, MapServiceError(fmt.Errorf("%w: engine must be a supported database engine (postgresql, mysql, sqlserver)", service.ErrInvalidParameters))
			}
			canonicalEngine = canonical
		}

		var reqVCPU float64
		if input.VCPU != nil {
			if *input.VCPU <= 0 {
				return nil, nil, MapServiceError(fmt.Errorf("%w: vcpu must be a positive number", service.ErrInvalidParameters))
			}
			reqVCPU = *input.VCPU
		}

		var reqRAMGB float64
		if input.RAMGB != nil {
			if *input.RAMGB <= 0 {
				return nil, nil, MapServiceError(fmt.Errorf("%w: ram_gb must be a positive number", service.ErrInvalidParameters))
			}
			reqRAMGB = *input.RAMGB
		}

		var reqStorageGB float64
		if input.StorageGB != nil {
			if *input.StorageGB <= 0 || *input.StorageGB > maxAllowedDatabaseStorageGB {
				return nil, nil, MapServiceError(fmt.Errorf("%w: storage_gb must be a positive number up to 1000000", service.ErrInvalidParameters))
			}
			reqStorageGB = *input.StorageGB
		}

		if input.IOPS != nil && *input.IOPS <= 0 {
			return nil, nil, MapServiceError(fmt.Errorf("%w: iops must be a positive integer", service.ErrInvalidParameters))
		}

		var multiAZ bool
		if input.MultiAZ != nil {
			multiAZ = *input.MultiAZ
		}

		target := service.MatchTarget{
			Engine:            canonicalEngine,
			VCPU:              reqVCPU,
			RAMGB:             reqRAMGB,
			DatabaseStorageGB: reqStorageGB,
			DatabaseIOPS:      input.IOPS,
			MultiAZ:           multiAZ,
			StorageFamily:     input.StorageFamily,
			Category:          "database_rdbms",
		}

		compRes, err := pricingSvc.Compare(ctx, "database_rdbms", region, target)
		if err != nil {
			return nil, nil, MapServiceError(err)
		}

		for _, w := range compRes.Warnings {
			warnings = append(warnings, ProviderWarning{
				Provider: w.Provider,
				Code:     w.Code,
				Message:  w.Message,
			})
		}

		var results []DatabaseResultEntry
		for _, item := range compRes.Results {
			results = append(results, DatabaseResultEntry{
				Provider:          item.Provider,
				SkuID:             item.SkuID,
				MatchedSpec:       item.MatchedDatabase,
				MatchQuality:      item.MatchQuality,
				MatchDeltaPct:     item.MatchDeltaPct,
				MissingAttributes: item.MissingAttributes,
				Price: PriceDetail{
					Amount:   item.HourlyCost,
					Unit:     item.Unit,
					Currency: currency,
				},
				NormalizedHourlyUSD: item.HourlyCost,
				FetchedAt:           item.FetchedAt,
				Stale:               item.Stale,
			})
		}

		queryMeta := DatabaseQueryMeta{
			Category:      "database_rdbms",
			Region:        region,
			Currency:      reqCurrency,
			Engine:        canonicalEngine,
			StorageFamily: input.StorageFamily,
		}
		if reqVCPU > 0 {
			queryMeta.VCPU = &reqVCPU
		}
		if reqRAMGB > 0 {
			queryMeta.RAMGB = &reqRAMGB
		}
		if reqStorageGB > 0 {
			queryMeta.StorageGB = &reqStorageGB
		}
		if input.IOPS != nil {
			queryMeta.IOPS = input.IOPS
		}
		if input.MultiAZ != nil {
			queryMeta.MultiAZ = input.MultiAZ
		}

		resp := &DatabaseComparisonResponse{
			Meta: ResponseMeta{
				APIVersion:  "v1",
				GeneratedAt: time.Now().UTC(),
				Query:       queryMeta,
			},
			Results:  results,
			Warnings: warnings,
		}

		return nil, resp, nil
	}
}

// handleCompareDatabaseNoSQL creates the tool handler for compare_database_nosql.
func handleCompareDatabaseNoSQL(pricingSvc *service.PricingService) sdk.ToolHandlerFor[CompareDatabaseNoSQLInput, any] {
	return func(ctx context.Context, req *sdk.CallToolRequest, input CompareDatabaseNoSQLInput) (*sdk.CallToolResult, any, error) {
		if pricingSvc == nil {
			return nil, nil, errors.New("pricing service unavailable")
		}

		region := input.Region
		if region == "" {
			region = "us-east"
		}

		warnings := defaultStage3Warnings()

		currency := input.Currency
		reqCurrency := currency
		if reqCurrency == "" {
			reqCurrency = "USD"
		}
		if currency != "" && currency != "USD" {
			warnings = append(warnings, ProviderWarning{
				Provider: "system",
				Code:     "non_usd_currency_unsupported",
				Message:  "Currency conversion is not yet supported. Prices are returned in USD.",
			})
		}
		currency = "USD"

		var reqReads float64
		if input.ReadUnits != nil {
			reqReads = *input.ReadUnits
		}
		var reqWrites float64
		if input.WriteUnits != nil {
			reqWrites = *input.WriteUnits
		}
		var reqStorageGB float64
		if input.StorageGB != nil {
			reqStorageGB = *input.StorageGB
		}
		var multiRegion bool
		if input.MultiRegion != nil {
			multiRegion = *input.MultiRegion
		}

		nosqlAttr, err := validateDatabaseNoSQLParams(input.DataModel, input.PricingMode, reqReads, reqWrites, reqStorageGB, input.StorageClass, multiRegion, "")
		if err != nil {
			return nil, nil, MapServiceError(err)
		}

		target := service.MatchTarget{
			DataModel:        nosqlAttr.DataModel,
			PricingMode:      nosqlAttr.PricingMode,
			ReadUnits:        nosqlAttr.ReadUnits,
			WriteUnits:       nosqlAttr.WriteUnits,
			NoSQLStorageGB:   nosqlAttr.StorageGB,
			StorageClass:     nosqlAttr.StorageClass,
			NoSQLMultiRegion: nosqlAttr.MultiRegion,
			Category:         "database_nosql",
		}

		compRes, err := pricingSvc.Compare(ctx, "database_nosql", region, target)
		if err != nil {
			return nil, nil, MapServiceError(err)
		}

		for _, w := range compRes.Warnings {
			warnings = append(warnings, ProviderWarning{
				Provider: w.Provider,
				Code:     w.Code,
				Message:  w.Message,
			})
		}

		var results []DatabaseNoSQLResultEntry
		for _, item := range compRes.Results {
			results = append(results, DatabaseNoSQLResultEntry{
				Provider:          item.Provider,
				SkuID:             item.SkuID,
				MatchedSpec:       item.MatchedNoSQL,
				MatchQuality:      item.MatchQuality,
				MatchDeltaPct:     item.MatchDeltaPct,
				MissingAttributes: item.MissingAttributes,
				Price: PriceDetail{
					Amount:   item.HourlyCost,
					Unit:     item.Unit,
					Currency: currency,
				},
				NormalizedHourlyUSD: item.HourlyCost,
				FetchedAt:           item.FetchedAt,
				Stale:               item.Stale,
			})
		}

		queryMeta := DatabaseNoSQLQueryMeta{
			Category:     "database_nosql",
			Region:       region,
			Currency:     reqCurrency,
			DataModel:    nosqlAttr.DataModel,
			PricingMode:  nosqlAttr.PricingMode,
			StorageClass: input.StorageClass,
		}
		if input.ReadUnits != nil {
			queryMeta.ReadUnits = input.ReadUnits
		}
		if input.WriteUnits != nil {
			queryMeta.WriteUnits = input.WriteUnits
		}
		if input.StorageGB != nil {
			queryMeta.StorageGB = input.StorageGB
		}
		if input.MultiRegion != nil {
			queryMeta.MultiRegion = input.MultiRegion
		}

		resp := &DatabaseNoSQLComparisonResponse{
			Meta: ResponseMeta{
				APIVersion:  "v1",
				GeneratedAt: time.Now().UTC(),
				Query:       queryMeta,
			},
			Results:  results,
			Warnings: warnings,
		}

		return nil, resp, nil
	}
}

// handleCompareKubernetes creates the tool handler for compare_kubernetes.
func handleCompareKubernetes(pricingSvc *service.PricingService) sdk.ToolHandlerFor[CompareKubernetesInput, any] {
	return func(ctx context.Context, req *sdk.CallToolRequest, input CompareKubernetesInput) (*sdk.CallToolResult, any, error) {
		if pricingSvc == nil {
			return nil, nil, errors.New("pricing service unavailable")
		}

		region := input.Region
		if region == "" {
			region = "us-east"
		}

		warnings := defaultStage3Warnings()

		currency := input.Currency
		reqCurrency := currency
		if reqCurrency == "" {
			reqCurrency = "USD"
		}
		if currency != "" && currency != "USD" {
			warnings = append(warnings, ProviderWarning{
				Provider: "system",
				Code:     "non_usd_currency_unsupported",
				Message:  "Currency conversion is not yet supported. Prices are returned in USD.",
			})
		}
		currency = "USD"

		k8sAttr, err := validateKubernetesParams(input.Tier, input.ClusterTopology, "")
		if err != nil {
			return nil, nil, MapServiceError(err)
		}

		target := service.MatchTarget{
			Category:        "kubernetes",
			KubernetesTier:  k8sAttr.Tier,
			ClusterTopology: k8sAttr.ClusterTopology,
		}

		compRes, err := pricingSvc.Compare(ctx, "kubernetes", region, target)
		if err != nil {
			return nil, nil, MapServiceError(err)
		}

		for _, w := range compRes.Warnings {
			warnings = append(warnings, ProviderWarning{
				Provider: w.Provider,
				Code:     w.Code,
				Message:  w.Message,
			})
		}

		var results []KubernetesResultEntry
		for _, item := range compRes.Results {
			results = append(results, KubernetesResultEntry{
				Provider:          item.Provider,
				SkuID:             item.SkuID,
				MatchedSpec:       item.MatchedKubernetes,
				MatchQuality:      item.MatchQuality,
				MatchDeltaPct:     item.MatchDeltaPct,
				MissingAttributes: item.MissingAttributes,
				Price: PriceDetail{
					Amount:   item.PriceAmount,
					Unit:     item.Unit,
					Currency: currency,
				},
				NormalizedHourlyUSD: item.HourlyCost,
				FetchedAt:           item.FetchedAt,
				Stale:               item.Stale,
			})
		}

		queryMeta := KubernetesQueryMeta{
			Category:        "kubernetes",
			Region:          region,
			Currency:        reqCurrency,
			Tier:            string(k8sAttr.Tier),
			ClusterTopology: string(k8sAttr.ClusterTopology),
		}

		resp := &KubernetesComparisonResponse{
			Meta: ResponseMeta{
				APIVersion:  "v1",
				GeneratedAt: time.Now().UTC(),
				Query:       queryMeta,
			},
			Results:  results,
			Warnings: warnings,
		}

		return nil, resp, nil
	}
}

// handleCompareServerless creates the tool handler for compare_serverless.
func handleCompareServerless(pricingSvc *service.PricingService) sdk.ToolHandlerFor[CompareServerlessInput, any] {
	return func(ctx context.Context, req *sdk.CallToolRequest, input CompareServerlessInput) (*sdk.CallToolResult, any, error) {
		if pricingSvc == nil {
			return nil, nil, errors.New("pricing service unavailable")
		}

		region := input.Region
		if region == "" {
			region = "us-east"
		}

		warnings := defaultStage3Warnings()

		currency := input.Currency
		reqCurrency := currency
		if reqCurrency == "" {
			reqCurrency = "USD"
		}
		if currency != "" && currency != "USD" {
			warnings = append(warnings, ProviderWarning{
				Provider: "system",
				Code:     "non_usd_currency_unsupported",
				Message:  "Currency conversion is not yet supported. Prices are returned in USD.",
			})
		}
		currency = "USD"

		workload, err := validateServerlessRequirements(ServerlessRequirements{
			Architecture:        input.Architecture,
			Tier:                input.Tier,
			RequestsPerMonth:    input.RequestsPerMonth,
			MemoryMB:            input.MemoryMB,
			ExecutionDurationMS: input.ExecutionDurationMS,
		})
		if err != nil {
			return nil, nil, MapServiceError(err)
		}

		target := service.MatchTarget{
			Category:           "serverless",
			ServerlessWorkload: *workload,
		}

		compRes, err := pricingSvc.Compare(ctx, "serverless", region, target)
		if err != nil {
			return nil, nil, MapServiceError(err)
		}

		for _, w := range compRes.Warnings {
			warnings = append(warnings, ProviderWarning{
				Provider: w.Provider,
				Code:     w.Code,
				Message:  w.Message,
			})
		}

		var results []ServerlessResultEntry
		for _, item := range compRes.Results {
			results = append(results, ServerlessResultEntry{
				Provider:          item.Provider,
				SkuID:             item.SkuID,
				MatchedSpec:       item.MatchedServerless,
				MatchQuality:      item.MatchQuality,
				MatchDeltaPct:     item.MatchDeltaPct,
				MissingAttributes: item.MissingAttributes,
				Price: PriceDetail{
					Amount:   item.PriceAmount,
					Unit:     item.Unit,
					Currency: currency,
				},
				NormalizedHourlyUSD:  item.HourlyCost,
				NormalizedMonthlyUSD: item.MonthlyCost,
				FetchedAt:            item.FetchedAt,
				Stale:                item.Stale,
			})
		}

		queryMeta := ServerlessQueryMeta{
			Category:            "serverless",
			Region:              region,
			Currency:            reqCurrency,
			Architecture:        workload.Architecture,
			Tier:                workload.Tier,
			RequestsPerMonth:    input.RequestsPerMonth,
			MemoryMB:            input.MemoryMB,
			ExecutionDurationMS: input.ExecutionDurationMS,
		}

		resp := &ServerlessComparisonResponse{
			Meta: ResponseMeta{
				APIVersion:  "v1",
				GeneratedAt: time.Now().UTC(),
				Query:       queryMeta,
			},
			Results:  results,
			Warnings: warnings,
		}

		return nil, resp, nil
	}
}

// handleCalculateWorkload creates the tool handler for calculate_workload.
func handleCalculateWorkload(pricingSvc *service.PricingService) sdk.ToolHandlerFor[CalculateWorkloadInput, any] {
	return func(ctx context.Context, req *sdk.CallToolRequest, input CalculateWorkloadInput) (*sdk.CallToolResult, any, error) {
		if pricingSvc == nil {
			return nil, nil, errors.New("pricing service unavailable")
		}

		// Alias-conflict validation: Database vs DatabaseRDBMS
		dbReq, err := service.ResolveAliasedField(input.Database, input.DatabaseRDBMS, "database", "database_rdbms")
		if err != nil {
			return nil, nil, MapServiceError(err)
		}

		if input.Compute == nil && input.Storage == nil && input.Network == nil && dbReq == nil && input.DatabaseNoSQL == nil && input.Kubernetes == nil && input.Serverless == nil {
			return nil, nil, MapServiceError(service.ErrNoCategoriesRequested)
		}

		var computeAttr *domain.ComputeAttributes
		if input.Compute != nil {
			var err error
			computeAttr, err = validateComputeRequirements(*input.Compute)
			if err != nil {
				return nil, nil, MapServiceError(err)
			}
		}

		var storageAttr *domain.StorageAttributes
		if input.Storage != nil {
			var err error
			storageAttr, err = validateStorageRequirements(*input.Storage)
			if err != nil {
				return nil, nil, MapServiceError(err)
			}
		}

		var networkAttr *domain.NetworkAttributes
		if input.Network != nil {
			var err error
			networkAttr, err = validateNetworkRequirements(*input.Network)
			if err != nil {
				return nil, nil, MapServiceError(err)
			}
		}

		var dbAttr *domain.DatabaseRDBMSAttributes
		if dbReq != nil {
			var err error
			dbAttr, err = validateDatabaseRDBMSRequirements(*dbReq)
			if err != nil {
				return nil, nil, MapServiceError(err)
			}
		}

		var nosqlAttr *domain.DatabaseNoSQLAttributes
		if input.DatabaseNoSQL != nil {
			var err error
			nosqlAttr, err = validateDatabaseNoSQLRequirements(*input.DatabaseNoSQL)
			if err != nil {
				return nil, nil, MapServiceError(err)
			}
		}

		var k8sAttr *domain.KubernetesAttributes
		if input.Kubernetes != nil {
			var err error
			k8sAttr, err = validateKubernetesRequirements(*input.Kubernetes)
			if err != nil {
				return nil, nil, MapServiceError(err)
			}
		}

		var serverlessWorkload *service.ServerlessWorkload
		if input.Serverless != nil {
			var err error
			serverlessWorkload, err = validateServerlessRequirements(*input.Serverless)
			if err != nil {
				return nil, nil, MapServiceError(err)
			}
		}

		strictFamily := true
		if input.StrictFamily != nil {
			strictFamily = *input.StrictFamily
		}
		region := input.Region
		if region == "" {
			region = "us-east"
		}
		currency := input.Currency
		if currency == "" {
			currency = "USD"
		}

		warnings := defaultStage3Warnings()
		if currency != "USD" {
			warnings = append(warnings, ProviderWarning{
				Provider: "system",
				Code:     "non_usd_currency_unsupported",
				Message:  "Currency conversion is not yet supported. Prices are returned in USD.",
			})
		}

		svcReq := service.CalculateRequest{
			Region:        region,
			Currency:      "USD",
			StrictFamily:  strictFamily,
			Compute:       computeAttr,
			Storage:       storageAttr,
			Network:       networkAttr,
			DatabaseRDBMS: dbAttr,
			DatabaseNoSQL: nosqlAttr,
			Kubernetes:    k8sAttr,
			Serverless:    serverlessWorkload,
		}

		svcRes, err := pricingSvc.Calculate(ctx, svcReq)
		if err != nil {
			return nil, nil, MapServiceError(err)
		}

		for _, w := range svcRes.Warnings {
			warnings = append(warnings, ProviderWarning{
				Provider: w.Provider,
				Code:     w.Code,
				Message:  w.Message,
			})
		}

		var mappedResults []CalculateProviderResult
		for _, pr := range svcRes.Results {
			mappedCats := make(map[string]CalculateCategoryResult)
			for k, c := range pr.Categories {
				mappedCats[k] = CalculateCategoryResult{
					SkuID:               c.SkuID,
					MatchQuality:        c.MatchQuality,
					MatchDeltaPct:       c.MatchDeltaPct,
					MissingAttributes:   c.MissingAttributes,
					Stale:               c.Stale,
					NormalizedHourlyUSD: c.NormalizedHourlyUSD,
				}
			}

			mappedResults = append(mappedResults, CalculateProviderResult{
				Provider:                        pr.Provider,
				Categories:                      mappedCats,
				TotalNormalizedHourlyUSD:        pr.TotalNormalizedHourlyUSD,
				PartialTotalNormalizedHourlyUSD: pr.PartialTotalNormalizedHourlyUSD,
				Partial:                         pr.Partial,
			})
		}

		resp := &CalculateResponse{
			Meta: ResponseMeta{
				APIVersion:  "v1",
				GeneratedAt: time.Now().UTC(),
				Query:       input,
			},
			Results:  mappedResults,
			Warnings: warnings,
		}

		return nil, resp, nil
	}
}

// handleGetProviderStatus creates the tool handler for get_provider_status.
func handleGetProviderStatus(freshnessSvc *service.FreshnessService) sdk.ToolHandlerFor[GetProviderStatusInput, any] {
	return func(ctx context.Context, req *sdk.CallToolRequest, input GetProviderStatusInput) (*sdk.CallToolResult, any, error) {
		if freshnessSvc == nil {
			return nil, nil, errors.New("freshness service unavailable")
		}

		provider := strings.TrimSpace(input.Provider)
		if provider == "" {
			return nil, nil, MapServiceError(fmt.Errorf("%w: provider parameter is required", service.ErrInvalidParameters))
		}

		status, err := freshnessSvc.GetProviderStatus(ctx, provider)
		if err != nil {
			return nil, nil, MapServiceError(err)
		}

		return nil, status, nil
	}
}

// --- Get Compute Catalog Tool ---

// GetComputeCatalogInput represents input parameters for the get_compute_catalog MCP tool.
type GetComputeCatalogInput struct {
	Provider       string   `json:"provider,omitempty" jsonschema:"Cloud provider filter (e.g. aws, azure, gcp)"`
	Category       string   `json:"category,omitempty" jsonschema:"Instance category filter (e.g. general_purpose, compute_optimized, memory_optimized, gpu_accelerated, storage_optimized)"`
	InstanceFamily string   `json:"instance_family,omitempty" jsonschema:"Instance family filter (e.g. t3, c5, Standard_D)"`
	MinVCPU        *float64 `json:"min_vcpu,omitempty" jsonschema:"Minimum number of vCPUs"`
	MaxVCPU        *float64 `json:"max_vcpu,omitempty" jsonschema:"Maximum number of vCPUs"`
	MinMemoryGiB   *float64 `json:"min_memory_gib,omitempty" jsonschema:"Minimum RAM in GiB"`
	MaxMemoryGiB   *float64 `json:"max_memory_gib,omitempty" jsonschema:"Maximum RAM in GiB"`
	Limit          int32    `json:"limit,omitempty" jsonschema:"Maximum number of instances to return (default: 50, max: 200)"`
	Offset         int32    `json:"offset,omitempty" jsonschema:"Number of instances to skip for pagination (default: 0)"`
}

// ComputeCatalogOutput represents the result returned by the get_compute_catalog MCP tool.
type ComputeCatalogOutput struct {
	Count     int                         `json:"count"`
	Total     int64                       `json:"total"`
	Instances []domain.ComputeCatalogItem `json:"instances"`
}

func handleGetComputeCatalog(catalogSvc *service.CatalogService) sdk.ToolHandlerFor[GetComputeCatalogInput, any] {
	return func(ctx context.Context, req *sdk.CallToolRequest, input GetComputeCatalogInput) (*sdk.CallToolResult, any, error) {
		if catalogSvc == nil {
			return nil, nil, errors.New("catalog service unavailable")
		}

		filter := domain.CatalogFilter{
			Limit:  input.Limit,
			Offset: input.Offset,
		}
		if input.Provider != "" {
			filter.Provider = &input.Provider
		}
		if input.Category != "" {
			filter.Category = &input.Category
		}
		if input.InstanceFamily != "" {
			filter.InstanceFamily = &input.InstanceFamily
		}
		if input.MinVCPU != nil {
			if *input.MinVCPU < 0 {
				return nil, nil, fmt.Errorf("min_vcpu must be non-negative: %f", *input.MinVCPU)
			}
			filter.MinVCPU = input.MinVCPU
		}
		if input.MaxVCPU != nil {
			if *input.MaxVCPU < 0 {
				return nil, nil, fmt.Errorf("max_vcpu must be non-negative: %f", *input.MaxVCPU)
			}
			filter.MaxVCPU = input.MaxVCPU
		}
		if input.MinMemoryGiB != nil {
			if *input.MinMemoryGiB < 0 {
				return nil, nil, fmt.Errorf("min_memory_gib must be non-negative: %f", *input.MinMemoryGiB)
			}
			filter.MinMemoryGiB = input.MinMemoryGiB
		}
		if input.MaxMemoryGiB != nil {
			if *input.MaxMemoryGiB < 0 {
				return nil, nil, fmt.Errorf("max_memory_gib must be non-negative: %f", *input.MaxMemoryGiB)
			}
			filter.MaxMemoryGiB = input.MaxMemoryGiB
		}

		instances, total, err := catalogSvc.ListCatalogInstances(ctx, filter)
		if err != nil {
			return nil, nil, MapServiceError(err)
		}

		return nil, ComputeCatalogOutput{
			Count:     len(instances),
			Total:     total,
			Instances: instances,
		}, nil
	}
}

// RegisterTools registers all standard CloudVitta comparison, calculation, and catalog tools on the given MCP server.
func RegisterTools(server *sdk.Server, pricingSvc *service.PricingService, freshnessSvc *service.FreshnessService, cfg *serverConfig) {
	sdk.AddTool(server, &sdk.Tool{
		Name:        "compare_compute",
		Description: "Compare compute instance pricing across cloud providers for requested vCPU, RAM, and region requirements.",
	}, instrumentTool("compare_compute", cfg, handleCompareCompute(pricingSvc)))

	sdk.AddTool(server, &sdk.Tool{
		Name:        "compare_storage",
		Description: "Compare object and block storage pricing across cloud providers for specified capacity and storage class.",
	}, instrumentTool("compare_storage", cfg, handleCompareStorage(pricingSvc)))

	sdk.AddTool(server, &sdk.Tool{
		Name:        "compare_network",
		Description: "Compare outbound network data transfer pricing across cloud providers for specified egress volume.",
	}, instrumentTool("compare_network", cfg, handleCompareNetwork(pricingSvc)))

	sdk.AddTool(server, &sdk.Tool{
		Name:        "compare_database",
		Description: "Compare managed relational database (RDBMS) pricing across cloud providers for requested engine, compute, and storage specs.",
	}, instrumentTool("compare_database", cfg, handleCompareDatabase(pricingSvc)))

	sdk.AddTool(server, &sdk.Tool{
		Name:        "compare_database_nosql",
		Description: "Compare managed NoSQL database pricing across cloud providers for requested data model, throughput, and capacity.",
	}, instrumentTool("compare_database_nosql", cfg, handleCompareDatabaseNoSQL(pricingSvc)))

	sdk.AddTool(server, &sdk.Tool{
		Name:        "compare_kubernetes",
		Description: "Compare managed Kubernetes control plane pricing across cloud providers for requested tier and cluster topology.",
	}, instrumentTool("compare_kubernetes", cfg, handleCompareKubernetes(pricingSvc)))

	sdk.AddTool(server, &sdk.Tool{
		Name:        "compare_serverless",
		Description: "Compare serverless compute (FaaS) pricing across cloud providers for requested workload and architecture.",
	}, instrumentTool("compare_serverless", cfg, handleCompareServerless(pricingSvc)))

	sdk.AddTool(server, &sdk.Tool{
		Name:        "calculate_workload",
		Description: "Calculate composite multi-category workload costs across cloud providers with server-computed totals.",
	}, instrumentTool("calculate_workload", cfg, handleCalculateWorkload(pricingSvc)))

	sdk.AddTool(server, &sdk.Tool{
		Name:        "get_provider_status",
		Description: "Check data freshness, observation counts, staleness, and ingestion health for a cloud provider.",
	}, instrumentTool("get_provider_status", cfg, handleGetProviderStatus(freshnessSvc)))

	if cfg != nil && cfg.catalogSvc != nil {
		sdk.AddTool(server, &sdk.Tool{
			Name:        "get_compute_catalog",
			Description: "Returns cloud compute virtual machine inventory and hardware specifications filtered by provider, category, and compute requirements.",
		}, instrumentTool("get_compute_catalog", cfg, handleGetComputeCatalog(cfg.catalogSvc)))
	}
}

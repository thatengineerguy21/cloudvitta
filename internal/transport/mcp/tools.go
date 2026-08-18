package mcp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shopspring/decimal"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/service"
)

// maxAllowedStorageSizeGB represents the upper bound on single-request storage size (1 PB).
var maxAllowedStorageSizeGB = decimal.NewFromInt(1_000_000)

// maxAllowedNetworkEgressGB represents the upper bound on single-request egress size (10 PB).
var maxAllowedNetworkEgressGB = decimal.NewFromInt(10_000_000)

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

// ComputeComparisonMeta represents metadata in the compute comparison response.
type ComputeComparisonMeta struct {
	APIVersion  string                 `json:"api_version"`
	GeneratedAt time.Time              `json:"generated_at"`
	Query       map[string]interface{} `json:"query"`
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
	Meta     ComputeComparisonMeta `json:"meta"`
	Results  []ComputeResultEntry  `json:"results"`
	Warnings []ProviderWarning     `json:"warnings"`
}

// StorageComparisonMeta represents metadata in the storage comparison response.
type StorageComparisonMeta struct {
	APIVersion  string                 `json:"api_version"`
	GeneratedAt time.Time              `json:"generated_at"`
	Query       map[string]interface{} `json:"query"`
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
	Meta     StorageComparisonMeta `json:"meta"`
	Results  []StorageResultEntry  `json:"results"`
	Warnings []ProviderWarning     `json:"warnings"`
}

// NetworkComparisonMeta represents metadata in the network comparison response.
type NetworkComparisonMeta struct {
	APIVersion  string                 `json:"api_version"`
	GeneratedAt time.Time              `json:"generated_at"`
	Query       map[string]interface{} `json:"query"`
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
	Meta     NetworkComparisonMeta `json:"meta"`
	Results  []NetworkResultEntry  `json:"results"`
	Warnings []ProviderWarning     `json:"warnings"`
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

// CalculateMeta represents metadata for the calculate response.
type CalculateMeta struct {
	APIVersion  string                 `json:"api_version"`
	GeneratedAt time.Time              `json:"generated_at"`
	Request     CalculateWorkloadInput `json:"request"`
}

// CalculateResponse represents the full calculate response envelope.
type CalculateResponse struct {
	Meta     CalculateMeta             `json:"meta"`
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

// CalculateWorkloadInput defines parameters for calculate_workload tool.
type CalculateWorkloadInput struct {
	Region       string               `json:"region,omitempty" jsonschema:"Canonical region group (default: us-east)"`
	Currency     string               `json:"currency,omitempty" jsonschema:"Target currency code (default: USD)"`
	StrictFamily *bool                `json:"strict_family,omitempty" jsonschema:"Strict instance family matching (default: true)"`
	Compute      *ComputeRequirements `json:"compute,omitempty" jsonschema:"Compute requirements (vcpu, ram_gb, family)"`
	Storage      *StorageRequirements `json:"storage,omitempty" jsonschema:"Storage requirements (size_gb, storage_class)"`
	Network      *NetworkRequirements `json:"network,omitempty" jsonschema:"Network requirements (egress_gb, transfer_type)"`
}

// GetProviderStatusInput defines parameters for get_provider_status tool.
type GetProviderStatusInput struct {
	Provider string `json:"provider" jsonschema:"Cloud provider identifier (e.g. aws, azure, gcp, oracle, ibm, alibaba, digitalocean)"`
}

// defaultStage3Warnings returns the static warnings for providers scheduled for stage 3/4.
func defaultStage3Warnings() []ProviderWarning {
	return []ProviderWarning{
		{Provider: "oracle", Code: "not_yet_ingested", Message: "Oracle OCI ingestion lands in stage 3."},
		{Provider: "ibm", Code: "not_yet_ingested", Message: "IBM Cloud ingestion lands in stage 3."},
		{Provider: "alibaba", Code: "not_yet_ingested", Message: "Alibaba Cloud ingestion lands in stage 3."},
		{Provider: "digitalocean", Code: "not_yet_ingested", Message: "DigitalOcean ingestion lands in stage 3."},
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
				Code:     "currency_conversion_not_yet_supported",
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
				return nil, nil, fmt.Errorf("%w: vcpu must be a positive number", service.ErrInvalidParameters)
			}
			reqVCPU = *input.VCPU
		}

		var reqRAMGB float64
		if input.RAMGB != nil {
			if *input.RAMGB <= 0 {
				return nil, nil, fmt.Errorf("%w: ram_gb must be a positive number", service.ErrInvalidParameters)
			}
			reqRAMGB = *input.RAMGB
		}

		providers := service.SupportedProviders()
		var results []ComputeResultEntry
		var providerErrors int

		for _, prov := range providers {
			target := service.MatchTarget{
				VCPU:         reqVCPU,
				RAMGB:        reqRAMGB,
				Family:       input.Family,
				StrictFamily: strictFamily,
				Category:     "compute",
			}

			catResult, err := pricingSvc.MatchAndCalculate(ctx, prov, "compute", region, target)
			if err != nil {
				switch err {
				case service.ErrCategoryNotSupported:
					warnings = append(warnings, ProviderWarning{
						Provider: prov,
						Code:     "category_not_supported",
						Message:  "Compute category is not supported by " + prov,
					})
				case service.ErrNoMatchFound:
					warnings = append(warnings, ProviderWarning{
						Provider: prov,
						Code:     "no_match",
						Message:  "No compute SKU matched the requested spec within acceptable thresholds.",
					})
				default:
					providerErrors++
					warnings = append(warnings, ProviderWarning{
						Provider: prov,
						Code:     "fetch_failed",
						Message:  err.Error(),
					})
				}
				continue
			}

			if catResult == nil {
				warnings = append(warnings, ProviderWarning{
					Provider: prov,
					Code:     "no_data_available",
					Message:  "No compute pricing data available for this region.",
				})
				continue
			}

			obs := catResult.MatchResult.Observation
			results = append(results, ComputeResultEntry{
				Provider:          obs.Provider,
				SkuID:             obs.SkuID,
				MatchedSpec:       obs.Attributes,
				MatchQuality:      catResult.MatchResult.MatchQuality,
				MatchDeltaPct:     catResult.MatchResult.MatchDeltaPct,
				MissingAttributes: catResult.MatchResult.MissingAttributes,
				Price: PriceDetail{
					Amount:   obs.PriceAmount,
					Unit:     catResult.Unit,
					Currency: currency,
				},
				NormalizedHourlyUSD: catResult.HourlyCost,
				FetchedAt:           obs.FetchedAt,
				Stale:               catResult.Stale,
			})
		}

		if len(results) == 0 && providerErrors == len(providers) {
			return nil, nil, service.ErrProviderUnavailable
		}

		queryMeta := map[string]interface{}{
			"category": "compute",
			"region":   region,
			"currency": reqCurrency,
		}
		if reqVCPU > 0 {
			queryMeta["vcpu"] = reqVCPU
		}
		if reqRAMGB > 0 {
			queryMeta["ram_gb"] = reqRAMGB
		}
		if input.Family != "" {
			queryMeta["family"] = input.Family
		}
		if input.StrictFamily != nil {
			queryMeta["strict_family"] = strictFamily
		}

		resp := &ComputeComparisonResponse{
			Meta: ComputeComparisonMeta{
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
				Code:     "currency_conversion_not_yet_supported",
				Message:  "Currency conversion is not yet supported. Prices are returned in USD.",
			})
		}
		currency = "USD"

		sizeGB := decimal.NewFromInt(1)
		hasExplicitSize := false
		if input.SizeGB != nil {
			parsed := decimal.NewFromFloat(*input.SizeGB)
			if parsed.LessThanOrEqual(decimal.Zero) || parsed.GreaterThan(maxAllowedStorageSizeGB) {
				return nil, nil, fmt.Errorf("%w: size_gb must be a positive number no greater than 1000000", service.ErrInvalidParameters)
			}
			sizeGB = parsed
			hasExplicitSize = true
		}

		providers := service.SupportedProviders()
		var results []StorageResultEntry
		var providerErrors int

		for _, prov := range providers {
			sizeF, _ := sizeGB.Float64()
			target := service.MatchTarget{
				SizeGB:       sizeF,
				StorageClass: input.StorageClass,
				Category:     "storage",
			}

			catResult, err := pricingSvc.MatchAndCalculate(ctx, prov, "storage", region, target)
			if err != nil {
				switch err {
				case service.ErrCategoryNotSupported:
					warnings = append(warnings, ProviderWarning{
						Provider: prov,
						Code:     "category_not_supported",
						Message:  "Storage category is not supported by " + prov,
					})
				case service.ErrNoMatchFound:
					warnings = append(warnings, ProviderWarning{
						Provider: prov,
						Code:     "no_match",
						Message:  "No storage SKU matched the requested spec within acceptable thresholds.",
					})
				default:
					providerErrors++
					warnings = append(warnings, ProviderWarning{
						Provider: prov,
						Code:     "fetch_failed",
						Message:  err.Error(),
					})
				}
				continue
			}

			if catResult == nil {
				warnings = append(warnings, ProviderWarning{
					Provider: prov,
					Code:     "no_data_available",
					Message:  "No storage pricing data available for this region.",
				})
				continue
			}

			obs := catResult.MatchResult.Observation
			results = append(results, StorageResultEntry{
				Provider:          obs.Provider,
				SkuID:             obs.SkuID,
				MatchedSpec:       obs.StorageAttributes,
				MatchQuality:      catResult.MatchResult.MatchQuality,
				MatchDeltaPct:     catResult.MatchResult.MatchDeltaPct,
				MissingAttributes: catResult.MatchResult.MissingAttributes,
				Price: PriceDetail{
					Amount:   obs.PriceAmount,
					Unit:     catResult.Unit,
					Currency: currency,
				},
				MonthlyCostUSD: catResult.MonthlyCost,
				FetchedAt:      obs.FetchedAt,
				Stale:          catResult.Stale,
			})
		}

		if len(results) == 0 && providerErrors == len(providers) {
			return nil, nil, service.ErrProviderUnavailable
		}

		queryMeta := map[string]interface{}{
			"category": "storage",
			"region":   region,
			"currency": reqCurrency,
		}
		if hasExplicitSize {
			queryMeta["size_gb"] = sizeGB
		}
		if input.StorageClass != "" {
			queryMeta["storage_class"] = input.StorageClass
		}

		resp := &StorageComparisonResponse{
			Meta: StorageComparisonMeta{
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
				Code:     "currency_conversion_not_yet_supported",
				Message:  "Currency conversion is not yet supported. Prices are returned in USD.",
			})
		}
		currency = "USD"

		egressGB := decimal.NewFromInt(1)
		hasExplicitEgress := false
		if input.EgressGB != nil {
			parsed := decimal.NewFromFloat(*input.EgressGB)
			if parsed.LessThanOrEqual(decimal.Zero) || parsed.GreaterThan(maxAllowedNetworkEgressGB) {
				return nil, nil, fmt.Errorf("%w: egress_gb must be a positive number no greater than 10000000", service.ErrInvalidParameters)
			}
			egressGB = parsed
			hasExplicitEgress = true
		}

		providers := service.SupportedProviders()
		var results []NetworkResultEntry
		var providerErrors int

		for _, prov := range providers {
			egressF, _ := egressGB.Float64()
			target := service.MatchTarget{
				EgressGB:     egressF,
				TransferType: input.TransferType,
				Category:     "network",
			}

			catResult, err := pricingSvc.MatchAndCalculate(ctx, prov, "network", region, target)
			if err != nil {
				switch err {
				case service.ErrCategoryNotSupported:
					warnings = append(warnings, ProviderWarning{
						Provider: prov,
						Code:     "category_not_supported",
						Message:  "Network category is not supported by " + prov,
					})
				case service.ErrNoMatchFound:
					warnings = append(warnings, ProviderWarning{
						Provider: prov,
						Code:     "no_match",
						Message:  "No network SKU matched the requested spec within acceptable thresholds.",
					})
				default:
					providerErrors++
					warnings = append(warnings, ProviderWarning{
						Provider: prov,
						Code:     "fetch_failed",
						Message:  err.Error(),
					})
				}
				continue
			}

			if catResult == nil {
				warnings = append(warnings, ProviderWarning{
					Provider: prov,
					Code:     "no_data_available",
					Message:  "No network pricing data available for this region.",
				})
				continue
			}

			obs := catResult.MatchResult.Observation
			results = append(results, NetworkResultEntry{
				Provider:          obs.Provider,
				SkuID:             obs.SkuID,
				MatchedSpec:       obs.NetworkAttributes,
				MatchQuality:      catResult.MatchResult.MatchQuality,
				MatchDeltaPct:     catResult.MatchResult.MatchDeltaPct,
				MissingAttributes: catResult.MatchResult.MissingAttributes,
				Price: PriceDetail{
					Amount:   obs.PriceAmount,
					Unit:     catResult.Unit,
					Currency: currency,
				},
				MonthlyCostUSD: catResult.MonthlyCost,
				FetchedAt:      obs.FetchedAt,
				Stale:          catResult.Stale,
			})
		}

		if len(results) == 0 && providerErrors == len(providers) {
			return nil, nil, service.ErrProviderUnavailable
		}

		queryMeta := map[string]interface{}{
			"category": "network",
			"region":   region,
			"currency": reqCurrency,
		}
		if hasExplicitEgress {
			queryMeta["egress_gb"] = egressGB
		}
		if input.TransferType != "" {
			queryMeta["transfer_type"] = input.TransferType
		}

		resp := &NetworkComparisonResponse{
			Meta: NetworkComparisonMeta{
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

		if input.Compute == nil && input.Storage == nil && input.Network == nil {
			return nil, nil, service.ErrNoCategoriesRequested
		}

		if input.Compute != nil {
			if input.Compute.VCPU <= 0 {
				return nil, nil, fmt.Errorf("%w: compute.vcpu must be a positive number", service.ErrInvalidParameters)
			}
			if input.Compute.RAMGB <= 0 {
				return nil, nil, fmt.Errorf("%w: compute.ram_gb must be a positive number", service.ErrInvalidParameters)
			}
		}

		if input.Storage != nil {
			if input.Storage.SizeGB <= 0 {
				return nil, nil, fmt.Errorf("%w: storage.size_gb must be a positive number", service.ErrInvalidParameters)
			}
			if input.Storage.SizeGB > 1000000 {
				return nil, nil, fmt.Errorf("%w: storage.size_gb exceeds maximum limit of 1,000,000 GB (1 PB)", service.ErrInvalidParameters)
			}
		}

		if input.Network != nil {
			if input.Network.EgressGB < 0 {
				return nil, nil, fmt.Errorf("%w: network.egress_gb cannot be negative", service.ErrInvalidParameters)
			}
			if input.Network.EgressGB > 10000000 {
				return nil, nil, fmt.Errorf("%w: network.egress_gb exceeds maximum limit of 10,000,000 GB (10 PB)", service.ErrInvalidParameters)
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

		var warnings []ProviderWarning
		if currency != "USD" {
			warnings = append(warnings, ProviderWarning{
				Provider: "system",
				Code:     "currency_conversion_not_yet_supported",
				Message:  "Currency conversion is not yet supported. Prices are returned in USD.",
			})
		}

		var computeAttr *domain.ComputeAttributes
		if input.Compute != nil {
			computeAttr = &domain.ComputeAttributes{
				VCPU:   input.Compute.VCPU,
				RAMGB:  input.Compute.RAMGB,
				Family: input.Compute.Family,
			}
		}

		var storageAttr *domain.StorageAttributes
		if input.Storage != nil {
			storageAttr = &domain.StorageAttributes{
				SizeGB:       input.Storage.SizeGB,
				StorageClass: input.Storage.StorageClass,
			}
		}

		var networkAttr *domain.NetworkAttributes
		if input.Network != nil {
			networkAttr = &domain.NetworkAttributes{
				EgressGB:     input.Network.EgressGB,
				TransferType: input.Network.TransferType,
			}
		}

		svcReq := service.CalculateRequest{
			Region:       region,
			Currency:     "USD",
			StrictFamily: strictFamily,
			Compute:      computeAttr,
			Storage:      storageAttr,
			Network:      networkAttr,
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

		warnings = append(warnings, defaultStage3Warnings()...)

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
			Meta: CalculateMeta{
				APIVersion:  "v1",
				GeneratedAt: time.Now().UTC(),
				Request:     input,
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
			return nil, nil, fmt.Errorf("%w: provider parameter is required", service.ErrInvalidParameters)
		}

		status, err := freshnessSvc.GetProviderStatus(ctx, provider)
		if err != nil {
			return nil, nil, MapServiceError(err)
		}

		return nil, status, nil
	}
}

// RegisterTools registers all 5 standard CloudVitta comparison and calculation tools on the given MCP server.
func RegisterTools(server *sdk.Server, pricingSvc *service.PricingService, freshnessSvc *service.FreshnessService) {
	sdk.AddTool(server, &sdk.Tool{
		Name:        "compare_compute",
		Description: "Compare compute instance pricing across cloud providers for requested vCPU, RAM, and region requirements.",
	}, handleCompareCompute(pricingSvc))

	sdk.AddTool(server, &sdk.Tool{
		Name:        "compare_storage",
		Description: "Compare object and block storage pricing across cloud providers for specified capacity and storage class.",
	}, handleCompareStorage(pricingSvc))

	sdk.AddTool(server, &sdk.Tool{
		Name:        "compare_network",
		Description: "Compare outbound network data transfer pricing across cloud providers for specified egress volume.",
	}, handleCompareNetwork(pricingSvc))

	sdk.AddTool(server, &sdk.Tool{
		Name:        "calculate_workload",
		Description: "Calculate composite multi-category workload costs across cloud providers with server-computed totals.",
	}, handleCalculateWorkload(pricingSvc))

	sdk.AddTool(server, &sdk.Tool{
		Name:        "get_provider_status",
		Description: "Check data freshness, observation counts, staleness, and ingestion health for a cloud provider.",
	}, handleGetProviderStatus(freshnessSvc))
}

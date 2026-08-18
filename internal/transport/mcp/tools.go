package mcp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/shopspring/decimal"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
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

// defaultStage3Warnings returns static warnings for providers scheduled for stage 3/4.
func defaultStage3Warnings() []ProviderWarning {
	return []ProviderWarning{
		{Provider: "oracle", Code: "not_yet_ingested", Message: "Oracle OCI ingestion lands in stage 3."},
		{Provider: "ibm", Code: "not_yet_ingested", Message: "IBM Cloud ingestion lands in stage 3."},
		{Provider: "alibaba", Code: "not_yet_ingested", Message: "Alibaba Cloud ingestion lands in stage 3."},
		{Provider: "digitalocean", Code: "not_yet_ingested", Message: "DigitalOcean ingestion lands in stage 3."},
	}
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
				Code:     "currency_conversion_not_yet_supported",
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
				Code:     "currency_conversion_not_yet_supported",
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

// handleCalculateWorkload creates the tool handler for calculate_workload.
func handleCalculateWorkload(pricingSvc *service.PricingService) sdk.ToolHandlerFor[CalculateWorkloadInput, any] {
	maxStorageF, _ := maxAllowedStorageSizeGB.Float64()
	maxNetworkF, _ := maxAllowedNetworkEgressGB.Float64()

	return func(ctx context.Context, req *sdk.CallToolRequest, input CalculateWorkloadInput) (*sdk.CallToolResult, any, error) {
		if pricingSvc == nil {
			return nil, nil, errors.New("pricing service unavailable")
		}

		if input.Compute == nil && input.Storage == nil && input.Network == nil {
			return nil, nil, MapServiceError(service.ErrNoCategoriesRequested)
		}

		if input.Compute != nil {
			if input.Compute.VCPU <= 0 {
				return nil, nil, MapServiceError(fmt.Errorf("%w: compute.vcpu must be a positive number", service.ErrInvalidParameters))
			}
			if input.Compute.RAMGB <= 0 {
				return nil, nil, MapServiceError(fmt.Errorf("%w: compute.ram_gb must be a positive number", service.ErrInvalidParameters))
			}
		}

		if input.Storage != nil {
			if input.Storage.SizeGB <= 0 {
				return nil, nil, MapServiceError(fmt.Errorf("%w: storage.size_gb must be a positive number", service.ErrInvalidParameters))
			}
			if input.Storage.SizeGB > maxStorageF {
				return nil, nil, MapServiceError(fmt.Errorf("%w: storage.size_gb exceeds maximum limit of %s GB (1 PB)", service.ErrInvalidParameters, maxAllowedStorageSizeGB.String()))
			}
		}

		if input.Network != nil {
			if input.Network.EgressGB < 0 {
				return nil, nil, MapServiceError(fmt.Errorf("%w: network.egress_gb cannot be negative", service.ErrInvalidParameters))
			}
			if input.Network.EgressGB > maxNetworkF {
				return nil, nil, MapServiceError(fmt.Errorf("%w: network.egress_gb exceeds maximum limit of %s GB (10 PB)", service.ErrInvalidParameters, maxAllowedNetworkEgressGB.String()))
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

// RegisterTools registers all 5 standard CloudVitta comparison and calculation tools on the given MCP server.
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
		Name:        "calculate_workload",
		Description: "Calculate composite multi-category workload costs across cloud providers with server-computed totals.",
	}, instrumentTool("calculate_workload", cfg, handleCalculateWorkload(pricingSvc)))

	sdk.AddTool(server, &sdk.Tool{
		Name:        "get_provider_status",
		Description: "Check data freshness, observation counts, staleness, and ingestion health for a cloud provider.",
	}, instrumentTool("get_provider_status", cfg, handleGetProviderStatus(freshnessSvc)))
}

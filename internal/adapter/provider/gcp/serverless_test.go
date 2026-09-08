package gcp_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/thatengineerguy21/CloudVitta/internal/adapter/provider/gcp"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/quarantine"
)

func TestNormalize_GCPServerlessGolden(t *testing.T) {
	fixedTime := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)

	f, err := os.Open("../../../../testdata/golden/gcp/serverless.json")
	if err != nil {
		t.Fatalf("failed to open golden file: %v", err)
	}
	defer func() { _ = f.Close() }()

	res, _, err := gcp.Normalize(f, fixedTime)
	if err != nil {
		t.Fatalf("Normalize() unexpected error: %v", err)
	}
	obs := res.Observations

	if len(obs) != 3 {
		t.Fatalf("expected 3 observations (request_fee, duration_fee_cpu, duration_fee_memory), got %d", len(obs))
	}

	componentTypes := make(map[string]bool)
	for _, o := range obs {
		if o.Provider != "gcp" {
			t.Errorf("expected Provider gcp, got %s", o.Provider)
		}
		if o.ServiceCategory != "serverless" {
			t.Errorf("expected ServiceCategory serverless, got %s", o.ServiceCategory)
		}
		if o.PriceCurrency != "USD" {
			t.Errorf("expected PriceCurrency USD, got %s", o.PriceCurrency)
		}
		if o.Region != "us-central1" || o.RegionGroup != "us-central" {
			t.Errorf("unexpected region/group: %s / %s", o.Region, o.RegionGroup)
		}
		if o.ServerlessRateAttributes.Architecture != domain.ArchitectureX86_64 {
			t.Errorf("unexpected architecture: %s", o.ServerlessRateAttributes.Architecture)
		}
		componentTypes[o.ServerlessRateAttributes.ComponentType] = true
	}

	if !componentTypes[domain.ComponentTypeRequestFee] {
		t.Errorf("expected request_fee component observation")
	}
	if !componentTypes[domain.ComponentTypeDurationFeeCPU] {
		t.Errorf("expected duration_fee_cpu component observation")
	}
	if !componentTypes[domain.ComponentTypeDurationFeeMemory] {
		t.Errorf("expected duration_fee_memory component observation")
	}
}

func TestNormalize_GCPCloudRun(t *testing.T) {
	fixedTime := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)

	f, err := os.Open("../../../../testdata/golden/gcp/cloud_run.json")
	if err != nil {
		t.Fatalf("failed to open cloud_run golden file: %v", err)
	}
	defer func() { _ = f.Close() }()

	res, _, err := gcp.Normalize(f, fixedTime)
	if err != nil {
		t.Fatalf("Normalize() unexpected error: %v", err)
	}
	obs := res.Observations
	if len(obs) != 3 {
		t.Fatalf("expected 3 observations, got %d", len(obs))
	}

	for _, o := range obs {
		if o.Provider != "gcp" {
			t.Errorf("expected Provider gcp, got %s", o.Provider)
		}
		if o.ServiceCategory != "serverless" {
			t.Errorf("expected ServiceCategory serverless, got %s", o.ServiceCategory)
		}
		if o.ServerlessRateAttributes.Architecture != domain.ArchitectureX86_64 {
			t.Errorf("expected Architecture x86_64, got %s", o.ServerlessRateAttributes.Architecture)
		}
	}

	// Requests
	reqObs := obs[0]
	if reqObs.ServerlessRateAttributes.ComponentType != domain.ComponentTypeRequestFee {
		t.Errorf("expected ComponentType request_fee, got %s", reqObs.ServerlessRateAttributes.ComponentType)
	}
	if reqObs.ServerlessRateAttributes.Unit != domain.UnitPerRequest {
		t.Errorf("expected Unit per_request, got %s", reqObs.ServerlessRateAttributes.Unit)
	}

	// CPU
	cpuObs := obs[1]
	if cpuObs.ServerlessRateAttributes.ComponentType != domain.ComponentTypeDurationFeeCPU {
		t.Errorf("expected ComponentType duration_fee_cpu, got %s", cpuObs.ServerlessRateAttributes.ComponentType)
	}
	if cpuObs.ServerlessRateAttributes.Unit != domain.UnitPerVCPUSecond {
		t.Errorf("expected Unit per_vcpu_second, got %s", cpuObs.ServerlessRateAttributes.Unit)
	}

	// Memory
	memObs := obs[2]
	if memObs.ServerlessRateAttributes.ComponentType != domain.ComponentTypeDurationFeeMemory {
		t.Errorf("expected ComponentType duration_fee_memory, got %s", memObs.ServerlessRateAttributes.ComponentType)
	}
	if memObs.ServerlessRateAttributes.Unit != domain.UnitPerGBSecond {
		t.Errorf("expected Unit per_gb_second, got %s", memObs.ServerlessRateAttributes.Unit)
	}
}

func TestNormalize_GCPServerless_UnmappedArchQuarantine(t *testing.T) {
	fixedTime := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)
	rawJSON := `{
		"skus": [
			{
				"name": "services/29E7-DA93-CA13/skus/SKU-GCP-CF-ARM-INVOCATIONS",
				"skuId": "SKU-GCP-CF-ARM-INVOCATIONS",
				"description": "Cloud Functions ARM Invocations",
				"category": {
					"serviceDisplayName": "Cloud Functions",
					"resourceFamily": "ApplicationServices",
					"resourceGroup": "Invocations",
					"usageType": "OnDemand"
				},
				"serviceRegions": [
					"us-central1"
				],
				"pricingInfo": [
					{
						"pricingExpression": {
							"usageUnit": "Calls",
							"usageUnitDescription": "invocations",
							"tieredRates": [
								{
									"startUsageAmount": 0,
									"unitPrice": {
										"currencyCode": "USD",
										"units": "0",
										"nanos": 400000
									}
								}
							]
						}
					}
				],
				"serviceProviderName": "Google"
			}
		]
	}`

	sink := &mockQuarantineSink{}
	res, _, err := gcp.Normalize(strings.NewReader(rawJSON), fixedTime, sink)
	if err != nil {
		t.Fatalf("Normalize() unexpected error: %v", err)
	}
	obs := res.Observations

	if len(obs) != 0 {
		t.Fatalf("expected 0 normalized observations for unmapped arch, got %d", len(obs))
	}

	if len(sink.items) != 1 {
		t.Fatalf("expected 1 quarantine sink item, got %d", len(sink.items))
	}

	item := sink.items[0]
	if item.Provider != "gcp" || item.Category != "serverless" || item.Kind != "serverless_architecture" || item.SkuID != "SKU-GCP-CF-ARM-INVOCATIONS" {
		t.Errorf("unexpected quarantine item: %+v", item)
	}
}

func TestNormalize_GCPServerless_UnmappedUnitQuarantine(t *testing.T) {
	fixedTime := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)
	rawJSON := `{
		"skus": [
			{
				"name": "services/29E7-DA93-CA13/skus/SKU-GCP-CF-UNKNOWN-UNIT",
				"skuId": "SKU-GCP-CF-UNKNOWN-UNIT",
				"description": "Cloud Functions Unknown Meter",
				"category": {
					"serviceDisplayName": "Cloud Functions",
					"resourceFamily": "ApplicationServices",
					"resourceGroup": "UnknownGroup",
					"usageType": "OnDemand"
				},
				"serviceRegions": [
					"us-central1"
				],
				"pricingInfo": [
					{
						"pricingExpression": {
							"usageUnit": "unknown_metric_unit",
							"usageUnitDescription": "unknown",
							"tieredRates": [
								{
									"startUsageAmount": 0,
									"unitPrice": {
										"currencyCode": "USD",
										"units": "1",
										"nanos": 0
									}
								}
							]
						}
					}
				],
				"serviceProviderName": "Google"
			}
		]
	}`

	sink := &mockQuarantineSink{}
	res, _, err := gcp.Normalize(strings.NewReader(rawJSON), fixedTime, sink)
	if err != nil {
		t.Fatalf("Normalize() unexpected error: %v", err)
	}
	obs := res.Observations

	if len(obs) != 0 {
		t.Fatalf("expected 0 normalized observations, got %d", len(obs))
	}
}

func TestNormalize_GCPServerless_FreeTierBypass(t *testing.T) {
	fixedTime := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)
	rawJSON := `{
		"skus": [
			{
				"name": "services/29E7-DA93-CA13/skus/SKU-GCP-CF-INVOCATIONS-TIERED",
				"skuId": "SKU-GCP-CF-INVOCATIONS-TIERED",
				"description": "Cloud Functions Invocations with Free Tier",
				"category": {
					"serviceDisplayName": "Cloud Functions",
					"resourceFamily": "ApplicationServices",
					"resourceGroup": "Invocations",
					"usageType": "OnDemand"
				},
				"serviceRegions": [
					"us-central1"
				],
				"pricingInfo": [
					{
						"pricingExpression": {
							"usageUnit": "Calls",
							"usageUnitDescription": "invocations",
							"tieredRates": [
								{
									"startUsageAmount": 0,
									"unitPrice": {
										"currencyCode": "USD",
										"units": "0",
										"nanos": 0
									}
								},
								{
									"startUsageAmount": 2000000,
									"unitPrice": {
										"currencyCode": "USD",
										"units": "0",
										"nanos": 400000
									}
								}
							]
						}
					}
				],
				"serviceProviderName": "Google"
			}
		]
	}`

	res, _, err := gcp.Normalize(strings.NewReader(rawJSON), fixedTime)
	if err != nil {
		t.Fatalf("Normalize() unexpected error: %v", err)
	}

	if len(res.Observations) != 1 {
		t.Fatalf("expected 1 observation extracting billable tier rate, got %d", len(res.Observations))
	}

	obs := res.Observations[0]
	if obs.PriceAmount.String() != "0.0004" {
		t.Errorf("expected PriceAmount 0.0004 from tier 1, got %s", obs.PriceAmount.String())
	}
	if obs.ServerlessRateAttributes.ComponentType != domain.ComponentTypeRequestFee {
		t.Errorf("expected request_fee component, got %s", obs.ServerlessRateAttributes.ComponentType)
	}
}

func TestNormalize_GCPServerless_AllocationRouting(t *testing.T) {
	fixedTime := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name              string
		desc              string
		resGroup          string
		usageUnit         string
		wantComponentType string
		wantUnit          string
	}{
		{
			name:              "Cloud Run functions CPU (Request-based billing)",
			desc:              "Cloud Run functions CPU (Request-based billing)",
			resGroup:          "CPU",
			usageUnit:         "vcpu.s",
			wantComponentType: domain.ComponentTypeDurationFeeCPU,
			wantUnit:          domain.UnitPerVCPUSecond,
		},
		{
			name:              "Cloud Run functions Memory (Request-based billing)",
			desc:              "Cloud Run functions Memory (Request-based billing)",
			resGroup:          "Memory",
			usageUnit:         "giby.s",
			wantComponentType: domain.ComponentTypeDurationFeeMemory,
			wantUnit:          domain.UnitPerGBSecond,
		},
		{
			name:              "Services CPU (Instance-based billing)",
			desc:              "Services CPU (Instance-based billing)",
			resGroup:          "CPU",
			usageUnit:         "vcpu.s",
			wantComponentType: domain.ComponentTypeDurationFeeCPU,
			wantUnit:          domain.UnitPerVCPUSecond,
		},
		{
			name:              "Services Memory (Instance-based billing)",
			desc:              "Services Memory (Instance-based billing)",
			resGroup:          "Memory",
			usageUnit:         "giby.s",
			wantComponentType: domain.ComponentTypeDurationFeeMemory,
			wantUnit:          domain.UnitPerGBSecond,
		},
		{
			name:              "Worker Pools CPU (Request-based billing)",
			desc:              "Worker Pools CPU (Request-based billing)",
			resGroup:          "CPU",
			usageUnit:         "vcpu.s",
			wantComponentType: domain.ComponentTypeDurationFeeCPU,
			wantUnit:          domain.UnitPerVCPUSecond,
		},
		{
			name:              "Worker Pools Memory (Request-based billing)",
			desc:              "Worker Pools Memory (Request-based billing)",
			resGroup:          "Memory",
			usageUnit:         "giby.s",
			wantComponentType: domain.ComponentTypeDurationFeeMemory,
			wantUnit:          domain.UnitPerGBSecond,
		},
		{
			name:              "Instances CPU (Request-based billing)",
			desc:              "Instances CPU (Request-based billing)",
			resGroup:          "CPU",
			usageUnit:         "vcpu.s",
			wantComponentType: domain.ComponentTypeDurationFeeCPU,
			wantUnit:          domain.UnitPerVCPUSecond,
		},
		{
			name:              "Instances Memory (Request-based billing)",
			desc:              "Instances Memory (Request-based billing)",
			resGroup:          "Memory",
			usageUnit:         "giby.s",
			wantComponentType: domain.ComponentTypeDurationFeeMemory,
			wantUnit:          domain.UnitPerGBSecond,
		},
		{
			name:              "Cloud Run Requests",
			desc:              "Cloud Run Requests",
			resGroup:          "Requests",
			usageUnit:         "requests",
			wantComponentType: domain.ComponentTypeRequestFee,
			wantUnit:          domain.UnitPerRequest,
		},
		{
			name:              "Cloud Run Request Count",
			desc:              "Cloud Run: Request Count",
			resGroup:          "Requests",
			usageUnit:         "requests",
			wantComponentType: domain.ComponentTypeRequestFee,
			wantUnit:          domain.UnitPerRequest,
		},
		{
			name:              "Cloud Functions Invocations",
			desc:              "Cloud Functions Invocations",
			resGroup:          "Invocations",
			usageUnit:         "Calls",
			wantComponentType: domain.ComponentTypeRequestFee,
			wantUnit:          domain.UnitPerRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rawJSON := `{
				"skus": [
					{
						"name": "services/152E-C115-5142/skus/TEST-SKU",
						"skuId": "TEST-SKU",
						"description": "` + tt.desc + `",
						"category": {
							"serviceDisplayName": "Cloud Run",
							"resourceFamily": "ApplicationServices",
							"resourceGroup": "` + tt.resGroup + `",
							"usageType": "OnDemand"
						},
						"serviceRegions": ["us-central1"],
						"pricingInfo": [
							{
								"pricingExpression": {
									"usageUnit": "` + tt.usageUnit + `",
									"tieredRates": [
										{
											"startUsageAmount": 0,
											"unitPrice": {
												"currencyCode": "USD",
												"units": "0",
												"nanos": 24000
											}
										}
									]
								}
							}
						],
						"serviceProviderName": "Google"
					}
				]
			}`

			sink := quarantine.NewMemorySink()
			res, _, err := gcp.Normalize(strings.NewReader(rawJSON), fixedTime, sink)
			if err != nil {
				t.Fatalf("Normalize() error = %v", err)
			}

			if sink.Count() != 0 {
				t.Fatalf("expected 0 quarantine items, got %d: %+v", sink.Count(), sink.Items())
			}

			if len(res.Observations) != 1 {
				t.Fatalf("expected 1 observation, got %d", len(res.Observations))
			}

			obs := res.Observations[0]
			if obs.ServerlessRateAttributes.ComponentType != tt.wantComponentType {
				t.Errorf("ComponentType = %q, want %q", obs.ServerlessRateAttributes.ComponentType, tt.wantComponentType)
			}
			if obs.ServerlessRateAttributes.Unit != tt.wantUnit {
				t.Errorf("Unit = %q, want %q", obs.ServerlessRateAttributes.Unit, tt.wantUnit)
			}
			if obs.ServerlessRateAttributes.Architecture != domain.ArchitectureX86_64 {
				t.Errorf("Architecture = %q, want %q", obs.ServerlessRateAttributes.Architecture, domain.ArchitectureX86_64)
			}
		})
	}
}

func TestNormalize_GCPServerless_OutOfScopeIgnored(t *testing.T) {
	fixedTime := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)

	outOfScopeSKUs := []struct {
		desc     string
		resGroup string
	}{
		{"Cloud Run Network Egress to Americas", "Network"},
		{"Data Transfer Out from Cloud Run", "DataTransfer"},
		{"Cloud Run Ingress", "Network"},
	}

	for _, tc := range outOfScopeSKUs {
		t.Run(tc.desc, func(t *testing.T) {
			rawJSON := `{
				"skus": [
					{
						"name": "services/152E-C115-5142/skus/TEST-OOS",
						"skuId": "TEST-OOS",
						"description": "` + tc.desc + `",
						"category": {
							"serviceDisplayName": "Cloud Run",
							"resourceFamily": "ApplicationServices",
							"resourceGroup": "` + tc.resGroup + `",
							"usageType": "OnDemand"
						},
						"serviceRegions": ["us-central1"],
						"pricingInfo": [
							{
								"pricingExpression": {
									"usageUnit": "GiBy",
									"tieredRates": [
										{
											"unitPrice": {
												"currencyCode": "USD",
												"units": "0",
												"nanos": 120000000
											}
										}
									]
								}
							}
						],
						"serviceProviderName": "Google"
					}
				]
			}`

			sink := quarantine.NewMemorySink()
			res, _, err := gcp.Normalize(strings.NewReader(rawJSON), fixedTime, sink)
			if err != nil {
				t.Fatalf("Normalize() error = %v", err)
			}

			if len(res.Observations) != 0 {
				t.Errorf("expected 0 observations for out-of-scope line item, got %d", len(res.Observations))
			}
			if res.IgnoredCount != 1 {
				t.Errorf("expected 1 ignored count, got %d", res.IgnoredCount)
			}
			if sink.Count() != 0 {
				t.Errorf("expected 0 quarantine items, got %d: %+v", sink.Count(), sink.Items())
			}
		})
	}
}

func TestNormalize_GCPServerless_RealisticMixUnderThreshold(t *testing.T) {
	fixedTime := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)

	// Mix of valid CPU, memory, request items plus out-of-scope network items
	rawJSON := `{
		"skus": [
			{
				"name": "services/152E-C115-5142/skus/CR-CPU-REQ",
				"skuId": "CR-CPU-REQ",
				"description": "Cloud Run functions CPU (Request-based billing)",
				"category": {"serviceDisplayName": "Cloud Run", "resourceGroup": "CPU", "usageType": "OnDemand"},
				"serviceRegions": ["us-central1"],
				"pricingInfo": [{"pricingExpression": {"usageUnit": "vcpu.s", "tieredRates": [{"unitPrice": {"currencyCode": "USD", "units": "0", "nanos": 24000}}]}}]
			},
			{
				"name": "services/152E-C115-5142/skus/CR-MEM-REQ",
				"skuId": "CR-MEM-REQ",
				"description": "Cloud Run functions Memory (Request-based billing)",
				"category": {"serviceDisplayName": "Cloud Run", "resourceGroup": "Memory", "usageType": "OnDemand"},
				"serviceRegions": ["us-central1"],
				"pricingInfo": [{"pricingExpression": {"usageUnit": "giby.s", "tieredRates": [{"unitPrice": {"currencyCode": "USD", "units": "0", "nanos": 2500}}]}}]
			},
			{
				"name": "services/152E-C115-5142/skus/CR-CPU-INST",
				"skuId": "CR-CPU-INST",
				"description": "Services CPU (Instance-based billing)",
				"category": {"serviceDisplayName": "Cloud Run", "resourceGroup": "CPU", "usageType": "OnDemand"},
				"serviceRegions": ["us-central1"],
				"pricingInfo": [{"pricingExpression": {"usageUnit": "vcpu.s", "tieredRates": [{"unitPrice": {"currencyCode": "USD", "units": "0", "nanos": 18000}}]}}]
			},
			{
				"name": "services/152E-C115-5142/skus/CR-MEM-INST",
				"skuId": "CR-MEM-INST",
				"description": "Services Memory (Instance-based billing)",
				"category": {"serviceDisplayName": "Cloud Run", "resourceGroup": "Memory", "usageType": "OnDemand"},
				"serviceRegions": ["us-central1"],
				"pricingInfo": [{"pricingExpression": {"usageUnit": "giby.s", "tieredRates": [{"unitPrice": {"currencyCode": "USD", "units": "0", "nanos": 2000}}]}}]
			},
			{
				"name": "services/152E-C115-5142/skus/CR-REQ",
				"skuId": "CR-REQ",
				"description": "Cloud Run: Request Count",
				"category": {"serviceDisplayName": "Cloud Run", "resourceGroup": "Requests", "usageType": "OnDemand"},
				"serviceRegions": ["us-central1"],
				"pricingInfo": [{"pricingExpression": {"usageUnit": "requests", "tieredRates": [{"unitPrice": {"currencyCode": "USD", "units": "0", "nanos": 400000}}]}}]
			},
			{
				"name": "services/152E-C115-5142/skus/CR-NET-EGRESS",
				"skuId": "CR-NET-EGRESS",
				"description": "Cloud Run Network Egress to Americas",
				"category": {"serviceDisplayName": "Cloud Run", "resourceGroup": "Network", "usageType": "OnDemand"},
				"serviceRegions": ["us-central1"],
				"pricingInfo": [{"pricingExpression": {"usageUnit": "GiBy", "tieredRates": [{"unitPrice": {"currencyCode": "USD", "units": "0", "nanos": 120000000}}]}}]
			}
		]
	}`

	sink := quarantine.NewMemorySink()
	res, _, err := gcp.Normalize(strings.NewReader(rawJSON), fixedTime, sink)
	if err != nil {
		t.Fatalf("Normalize() error = %v", err)
	}

	if len(res.Observations) != 5 {
		t.Errorf("expected 5 observations, got %d", len(res.Observations))
	}
	if res.IgnoredCount != 1 {
		t.Errorf("expected 1 ignored count for out-of-scope egress, got %d", res.IgnoredCount)
	}
	if sink.Count() != 0 {
		t.Errorf("expected 0 quarantine items, got %d: %+v", sink.Count(), sink.Items())
	}
}

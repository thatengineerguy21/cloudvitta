package gcp_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/thatengineerguy21/CloudVitta/internal/adapter/provider/gcp"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
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
	jsonInput := `{
		"skus": [
			{
				"name": "services/152E-C115-5142/skus/SKU-CR-REQUESTS",
				"skuId": "SKU-CR-REQUESTS",
				"description": "Cloud Run: Requests",
				"category": {
					"serviceDisplayName": "Cloud Run",
					"resourceFamily": "ApplicationServices",
					"resourceGroup": "Requests",
					"usageType": "OnDemand"
				},
				"serviceRegions": ["us-central1"],
				"pricingInfo": [{
					"pricingExpression": {
						"usageUnit": "requests",
						"tieredRates": [{"unitPrice": {"currencyCode": "USD", "units": "0", "nanos": 400000}}]
					}
				}]
			},
			{
				"name": "services/152E-C115-5142/skus/SKU-CR-CPU",
				"skuId": "SKU-CR-CPU",
				"description": "Cloud Run: CPU Allocation Time",
				"category": {
					"serviceDisplayName": "Cloud Run",
					"resourceFamily": "ApplicationServices",
					"resourceGroup": "CPU",
					"usageType": "OnDemand"
				},
				"serviceRegions": ["us-central1"],
				"pricingInfo": [{
					"pricingExpression": {
						"usageUnit": "vcpu.s",
						"tieredRates": [{"unitPrice": {"currencyCode": "USD", "units": "0", "nanos": 24000}}]
					}
				}]
			},
			{
				"name": "services/152E-C115-5142/skus/SKU-CR-MEM",
				"skuId": "SKU-CR-MEM",
				"description": "Cloud Run: Memory Allocation Time",
				"category": {
					"serviceDisplayName": "Cloud Run",
					"resourceFamily": "ApplicationServices",
					"resourceGroup": "Memory",
					"usageType": "OnDemand"
				},
				"serviceRegions": ["us-central1"],
				"pricingInfo": [{
					"pricingExpression": {
						"usageUnit": "giby.s",
						"tieredRates": [{"unitPrice": {"currencyCode": "USD", "units": "0", "nanos": 2500}}]
					}
				}]
			}
		]
	}`

	res, _, err := gcp.Normalize(strings.NewReader(jsonInput), fixedTime)
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

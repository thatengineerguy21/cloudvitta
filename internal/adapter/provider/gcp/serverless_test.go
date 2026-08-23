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

	obs, _, err := gcp.Normalize(f, fixedTime)
	if err != nil {
		t.Fatalf("Normalize() unexpected error: %v", err)
	}

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
	obs, _, err := gcp.Normalize(strings.NewReader(rawJSON), fixedTime, sink)
	if err != nil {
		t.Fatalf("Normalize() unexpected error: %v", err)
	}

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
	obs, _, err := gcp.Normalize(strings.NewReader(rawJSON), fixedTime, sink)
	if err != nil {
		t.Fatalf("Normalize() unexpected error: %v", err)
	}

	if len(obs) != 0 {
		t.Fatalf("expected 0 normalized observations, got %d", len(obs))
	}
}

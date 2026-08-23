package gcp_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/thatengineerguy21/CloudVitta/internal/adapter/provider/gcp"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/serverlessarchmap"
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

	if len(obs) != 2 {
		t.Fatalf("expected 2 observations, got %d", len(obs))
	}

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
		if o.ServerlessRateAttributes.Architecture != serverlessarchmap.ArchX86_64 {
			t.Errorf("unexpected architecture: %s", o.ServerlessRateAttributes.Architecture)
		}
		if o.ServerlessRateAttributes.ComponentType != domain.ComponentTypeRequestFee &&
			o.ServerlessRateAttributes.ComponentType != domain.ComponentTypeDurationFee {
			t.Errorf("unexpected component type: %s", o.ServerlessRateAttributes.ComponentType)
		}
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

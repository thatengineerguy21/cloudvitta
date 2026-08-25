package gcp_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/thatengineerguy21/CloudVitta/internal/adapter/provider/gcp"
	"github.com/thatengineerguy21/CloudVitta/internal/matching/kubernetestieremap"
	"github.com/thatengineerguy21/CloudVitta/internal/quarantine"
)

type mockQuarantineSink struct {
	items []quarantine.UnmappedItem
}

func (m *mockQuarantineSink) Record(ctx context.Context, item quarantine.UnmappedItem) error {
	m.items = append(m.items, item)
	return nil
}

func TestNormalize_GCPKubernetesGolden(t *testing.T) {
	fixedTime := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)

	f, err := os.Open("../../../../testdata/golden/gcp/kubernetes.json")
	if err != nil {
		t.Fatalf("failed to open golden file: %v", err)
	}
	defer func() { _ = f.Close() }()

	res, _, err := gcp.Normalize(f, fixedTime)
	if err != nil {
		t.Fatalf("Normalize() unexpected error: %v", err)
	}
	obs := res.Observations

	if len(obs) != 1 {
		t.Fatalf("expected 1 observation, got %d", len(obs))
	}

	for _, o := range obs {
		if o.Provider != "gcp" {
			t.Errorf("expected Provider gcp, got %s", o.Provider)
		}
		if o.ServiceCategory != "kubernetes" {
			t.Errorf("expected ServiceCategory kubernetes, got %s", o.ServiceCategory)
		}
		if o.PriceCurrency != "USD" {
			t.Errorf("expected PriceCurrency USD, got %s", o.PriceCurrency)
		}
		if o.Region != "us-east4" || o.RegionGroup != "us-east" {
			t.Errorf("unexpected region/group: %s / %s", o.Region, o.RegionGroup)
		}
		if o.KubernetesAttributes.Tier != kubernetestieremap.TierStandard {
			t.Errorf("expected tier %s, got %s", kubernetestieremap.TierStandard, o.KubernetesAttributes.Tier)
		}
	}
}

func TestNormalize_GCPKubernetes_UnmappedTierQuarantine(t *testing.T) {
	fixedTime := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)
	rawJSON := `{
		"skus": [
			{
				"name": "services/44CD-3C5E-2A4B/skus/SKU-GCP-GKE-UNKNOWN",
				"skuId": "SKU-GCP-GKE-UNKNOWN",
				"description": "GKE Quantum Computing Addon Fee",
				"category": {
					"serviceDisplayName": "Kubernetes Engine",
					"resourceFamily": "Compute",
					"resourceGroup": "Cluster",
					"usageType": "OnDemand"
				},
				"serviceRegions": [
					"us-east4"
				],
				"pricingInfo": [
					{
						"pricingExpression": {
							"usageUnit": "h",
							"usageUnitDescription": "hours",
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
		t.Fatalf("expected 0 observations for unmapped tier, got %d", len(obs))
	}

	if len(sink.items) != 1 {
		t.Fatalf("expected 1 quarantine sink item, got %d", len(sink.items))
	}

	item := sink.items[0]
	if item.Provider != "gcp" || item.Category != "kubernetes" || item.Kind != "kubernetes_tier" || item.SkuID != "SKU-GCP-GKE-UNKNOWN" {
		t.Errorf("unexpected quarantine item: %+v", item)
	}
}

package azure_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/thatengineerguy21/CloudVitta/internal/adapter/provider/azure"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
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

func TestNormalize_AzureKubernetesGolden(t *testing.T) {
	fixedTime := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)

	f, err := os.Open("../../../../testdata/golden/azure/kubernetes.json")
	if err != nil {
		t.Fatalf("failed to open golden file: %v", err)
	}
	defer func() { _ = f.Close() }()

	obs, _, err := azure.Normalize(f, fixedTime)
	if err != nil {
		t.Fatalf("Normalize() unexpected error: %v", err)
	}

	if len(obs) != 3 {
		t.Fatalf("expected 3 observations, got %d", len(obs))
	}

	tiersFound := make(map[domain.KubernetesTier]bool)
	for _, o := range obs {
		if o.Provider != "azure" {
			t.Errorf("expected Provider azure, got %s", o.Provider)
		}
		if o.ServiceCategory != "kubernetes" {
			t.Errorf("expected ServiceCategory kubernetes, got %s", o.ServiceCategory)
		}
		if o.PriceCurrency != "USD" {
			t.Errorf("expected PriceCurrency USD, got %s", o.PriceCurrency)
		}
		if o.Region != "eastus" || o.RegionGroup != "us-east" {
			t.Errorf("unexpected region/group: %s / %s", o.Region, o.RegionGroup)
		}
		tiersFound[o.KubernetesAttributes.Tier] = true
	}

	if !tiersFound[kubernetestieremap.TierFree] {
		t.Errorf("expected to find free tier observation")
	}
	if !tiersFound[kubernetestieremap.TierStandard] {
		t.Errorf("expected to find standard tier observation")
	}
	if !tiersFound[kubernetestieremap.TierExtendedSupport] {
		t.Errorf("expected to find extended_support tier observation")
	}
}

func TestNormalize_AzureKubernetes_UnmappedTierQuarantine(t *testing.T) {
	fixedTime := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)
	rawJSON := `{
		"Items": [
			{
				"currencyCode": "USD",
				"tierMinimumUnits": 0.0,
				"retailPrice": 0.99,
				"unitPrice": 0.99,
				"armRegionName": "eastus",
				"location": "US East",
				"meterId": "METER-AZURE-AKS-QUANTUM",
				"meterName": "Quantum Super Tier",
				"productId": "DZH318Z0AKS99",
				"skuId": "SKU-AZURE-AKS-QUANTUM",
				"productName": "Azure Kubernetes Service",
				"skuName": "Quantum Super Tier",
				"serviceName": "Azure Kubernetes Service",
				"serviceFamily": "Containers",
				"unitOfMeasure": "1 Hour",
				"type": "Consumption"
			}
		],
		"NextPageLink": "",
		"Count": 1
	}`

	sink := &mockQuarantineSink{}
	obs, _, err := azure.Normalize(strings.NewReader(rawJSON), fixedTime, sink)
	if err != nil {
		t.Fatalf("Normalize() unexpected error: %v", err)
	}

	if len(obs) != 0 {
		t.Fatalf("expected 0 observations for unmapped tier, got %d", len(obs))
	}

	if len(sink.items) != 1 {
		t.Fatalf("expected 1 quarantine sink item, got %d", len(sink.items))
	}

	item := sink.items[0]
	if item.Provider != "azure" || item.Category != "kubernetes" || item.Kind != "kubernetes_tier" || item.SkuID != "SKU-AZURE-AKS-QUANTUM" {
		t.Errorf("unexpected quarantine item: %+v", item)
	}
}

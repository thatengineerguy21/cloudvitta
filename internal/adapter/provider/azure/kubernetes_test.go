package azure_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/thatengineerguy21/CloudVitta/internal/adapter/provider/azure"
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

	res, _, err := azure.Normalize(f, fixedTime)
	if err != nil {
		t.Fatalf("Normalize() unexpected error: %v", err)
	}
	obs := res.Observations

	if len(obs) != 3 {
		t.Fatalf("expected 3 observations, got %d", len(obs))
	}

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
		if o.KubernetesAttributes.Tier != kubernetestieremap.TierFree &&
			o.KubernetesAttributes.Tier != kubernetestieremap.TierStandard &&
			o.KubernetesAttributes.Tier != kubernetestieremap.TierExtendedSupport {
			t.Errorf("unexpected tier: %s", o.KubernetesAttributes.Tier)
		}
	}
}

func TestNormalize_AzureKubernetes_UnmappedTierQuarantine(t *testing.T) {
	fixedTime := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)
	rawJSON := `{
		"Items": [
			{
				"currencyCode": "USD",
				"tierMinimumUnits": 0.0,
				"retailPrice": 0.50,
				"unitPrice": 0.50,
				"armRegionName": "eastus",
				"meterName": "Unknown Meter Tier",
				"skuName": "Unknown SKU Tier",
				"productName": "Azure Kubernetes Service",
				"serviceName": "Azure Kubernetes Service",
				"serviceFamily": "Compute",
				"unitOfMeasure": "1 Hour",
				"type": "Consumption",
				"isPrimaryMeterRegion": true,
				"skuId": "SKU-AZURE-AKS-UNKNOWN"
			}
		]
	}`

	sink := &mockQuarantineSink{}
	res, _, err := azure.Normalize(strings.NewReader(rawJSON), fixedTime, sink)
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
	if item.Provider != "azure" || item.Category != "kubernetes" || item.Kind != "kubernetes_tier" || !strings.HasPrefix(item.SkuID, "SKU-AZURE-AKS-UNKNOWN") {
		t.Errorf("unexpected quarantine item: %+v", item)
	}
}

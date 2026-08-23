package azure_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/thatengineerguy21/CloudVitta/internal/adapter/provider/azure"
	"github.com/thatengineerguy21/CloudVitta/internal/domain"
)

func TestNormalize_AzureServerlessGolden(t *testing.T) {
	fixedTime := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)

	f, err := os.Open("../../../../testdata/golden/azure/serverless.json")
	if err != nil {
		t.Fatalf("failed to open golden file: %v", err)
	}
	defer func() { _ = f.Close() }()

	obs, _, err := azure.Normalize(f, fixedTime)
	if err != nil {
		t.Fatalf("Normalize() unexpected error: %v", err)
	}

	if len(obs) != 4 {
		t.Fatalf("expected 4 observations, got %d", len(obs))
	}

	for _, o := range obs {
		if o.Provider != "azure" {
			t.Errorf("expected Provider azure, got %s", o.Provider)
		}
		if o.ServiceCategory != "serverless" {
			t.Errorf("expected ServiceCategory serverless, got %s", o.ServiceCategory)
		}
		if o.PriceCurrency != "USD" {
			t.Errorf("expected PriceCurrency USD, got %s", o.PriceCurrency)
		}
		if o.Region != "eastus" || o.RegionGroup != "us-east" {
			t.Errorf("unexpected region/group: %s / %s", o.Region, o.RegionGroup)
		}
		if o.ServerlessRateAttributes.Architecture != domain.ArchitectureX86_64 {
			t.Errorf("unexpected architecture: %s", o.ServerlessRateAttributes.Architecture)
		}
		if o.ServerlessRateAttributes.Tier != domain.ServerlessTierConsumption &&
			o.ServerlessRateAttributes.Tier != domain.ServerlessTierFlexConsumption {
			t.Errorf("unexpected tier: %s", o.ServerlessRateAttributes.Tier)
		}
		if o.ServerlessRateAttributes.ComponentType != domain.ComponentTypeRequestFee &&
			o.ServerlessRateAttributes.ComponentType != domain.ComponentTypeDurationFee {
			t.Errorf("unexpected component type: %s", o.ServerlessRateAttributes.ComponentType)
		}
	}
}

func TestNormalize_AzureServerless_UnmappedArchQuarantine(t *testing.T) {
	fixedTime := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)
	rawJSON := `{
		"Items": [
			{
				"currencyCode": "USD",
				"tierMinimumUnits": 0.0,
				"retailPrice": 0.000002,
				"unitPrice": 0.000002,
				"armRegionName": "eastus",
				"location": "US East",
				"effectiveStartDate": "2026-01-01T00:00:00Z",
				"meterId": "METER-AZURE-UNKNOWN-ARM",
				"meterName": "ARM64 Total Executions",
				"productId": "DZH318Z0FUNC1",
				"skuId": "SKU-AZURE-UNKNOWN-ARM",
				"productName": "Azure Functions",
				"skuName": "ARM64",
				"serviceName": "Azure Functions",
				"serviceId": "DZH313Z7FUNC1",
				"serviceFamily": "Compute",
				"unitOfMeasure": "10",
				"type": "Consumption",
				"isPrimaryMeterRegion": true,
				"armSkuName": "ARM64"
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
		t.Fatalf("expected 0 normalized observations for unmapped arch, got %d", len(obs))
	}

	if len(sink.items) != 1 {
		t.Fatalf("expected 1 quarantine sink item, got %d", len(sink.items))
	}

	item := sink.items[0]
	if item.Provider != "azure" || item.Category != "serverless" || item.Kind != "serverless_architecture" || item.SkuID != "SKU-AZURE-UNKNOWN-ARM" {
		t.Errorf("unexpected quarantine item: %+v", item)
	}
}

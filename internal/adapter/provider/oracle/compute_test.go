package oracle

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/thatengineerguy21/CloudVitta/internal/quarantine"
)

type memoryQuarantineSink struct {
	items []quarantine.UnmappedItem
}

func (s *memoryQuarantineSink) Record(ctx context.Context, item quarantine.UnmappedItem) error {
	s.items = append(s.items, item)
	return nil
}

func (s *memoryQuarantineSink) Count() int {
	return len(s.items)
}

func (s *memoryQuarantineSink) Flush(ctx context.Context) error {
	return nil
}

func TestNormalize_OCPUtoVCPU_x86_vs_Arm(t *testing.T) {
	jsonPayload := `{
		"items": [
			{
				"partNumber": "B93113",
				"displayName": "Compute - Virtual Machine - Standard - E4 - OCPU",
				"metricName": "OCPU Per Hour",
				"serviceCategory": "Compute - Virtual Machine",
				"prices": [
					{
						"currencyCode": "USD",
						"prices": [{"model": "PAY_AS_YOU_GO", "value": 0.025}]
					}
				],
				"regions": ["us-ashburn-1"]
			},
			{
				"partNumber": "B93114",
				"displayName": "Compute - Virtual Machine - Standard - E4 - Memory",
				"metricName": "Gigabyte Memory Per Hour",
				"serviceCategory": "Compute - Virtual Machine",
				"prices": [
					{
						"currencyCode": "USD",
						"prices": [{"model": "PAY_AS_YOU_GO", "value": 0.0015}]
					}
				],
				"regions": ["us-ashburn-1"]
			},
			{
				"partNumber": "B94178",
				"displayName": "Compute - Virtual Machine - Standard - A1 - OCPU",
				"metricName": "OCPU Per Hour",
				"serviceCategory": "Compute - Virtual Machine",
				"prices": [
					{
						"currencyCode": "USD",
						"prices": [{"model": "PAY_AS_YOU_GO", "value": 0.010}]
					}
				],
				"regions": ["us-ashburn-1"]
			},
			{
				"partNumber": "B94179",
				"displayName": "Compute - Virtual Machine - Standard - A1 - Memory",
				"metricName": "Gigabyte Memory Per Hour",
				"serviceCategory": "Compute - Virtual Machine",
				"prices": [
					{
						"currencyCode": "USD",
						"prices": [{"model": "PAY_AS_YOU_GO", "value": 0.0015}]
					}
				],
				"regions": ["us-ashburn-1"]
			}
		]
	}`

	now := time.Now().UTC()
	obs, err := Normalize(strings.NewReader(jsonPayload), now)
	if err != nil {
		t.Fatalf("Normalize failed: %v", err)
	}

	var foundE4_2vCPU, foundA1_1vCPU bool
	for _, o := range obs {
		if o.SkuID == "SKU-OCI-VM-STANDARD-E4-FLEX-2VCPU-8GB" {
			foundE4_2vCPU = true
			if o.Attributes.VCPU != 2 {
				t.Errorf("E4 1 OCPU expected 2 vCPU, got %v", o.Attributes.VCPU)
			}
			if o.Attributes.RAMGB != 8 {
				t.Errorf("E4 expected 8 GB RAM, got %v", o.Attributes.RAMGB)
			}
			// Price: 1 * 0.025 + 8 * 0.0015 = 0.037
			expectedPrice := decimal.RequireFromString("0.037")
			if !o.PriceAmount.Equal(expectedPrice) {
				t.Errorf("E4 price = %s, want %s", o.PriceAmount, expectedPrice)
			}
		}

		if o.SkuID == "SKU-OCI-VM-STANDARD-A1-FLEX-1VCPU-6GB" {
			foundA1_1vCPU = true
			if o.Attributes.VCPU != 1 {
				t.Errorf("A1 1 OCPU expected 1 vCPU (Arm 1:1), got %v", o.Attributes.VCPU)
			}
			if o.Attributes.RAMGB != 6 {
				t.Errorf("A1 expected 6 GB RAM, got %v", o.Attributes.RAMGB)
			}
			// Price: 1 * 0.010 + 6 * 0.0015 = 0.019
			expectedPrice := decimal.RequireFromString("0.019")
			if !o.PriceAmount.Equal(expectedPrice) {
				t.Errorf("A1 price = %s, want %s", o.PriceAmount, expectedPrice)
			}
		}
	}

	if !foundE4_2vCPU {
		t.Error("missing synthesized E4 2vCPU instance")
	}
	if !foundA1_1vCPU {
		t.Error("missing synthesized A1 1vCPU instance")
	}
}

func TestNormalize_FixedShapes(t *testing.T) {
	jsonPayload := `{
		"items": [
			{
				"partNumber": "B88317",
				"displayName": "Compute - Virtual Machine - Standard2.1",
				"metricName": "OCPU Per Hour",
				"serviceCategory": "Compute - Virtual Machine",
				"prices": [
					{
						"currencyCode": "USD",
						"prices": [{"model": "PAY_AS_YOU_GO", "value": 0.0638}]
					}
				],
				"regions": ["us-ashburn-1"]
			},
			{
				"partNumber": "B88319",
				"displayName": "Compute - Virtual Machine - Standard.E2.1",
				"metricName": "OCPU Per Hour",
				"serviceCategory": "Compute - Virtual Machine",
				"prices": [
					{
						"currencyCode": "USD",
						"prices": [{"model": "PAY_AS_YOU_GO", "value": 0.030}]
					}
				],
				"regions": ["us-phoenix-1"]
			}
		]
	}`

	now := time.Now().UTC()
	obs, err := Normalize(strings.NewReader(jsonPayload), now)
	if err != nil {
		t.Fatalf("Normalize failed: %v", err)
	}

	if len(obs) != 2 {
		t.Fatalf("expected 2 fixed observations, got %d", len(obs))
	}

	obs1 := obs[0]
	if obs1.Attributes.VCPU != 2 || obs1.Attributes.RAMGB != 15 {
		t.Errorf("Standard2.1 expected 2 vCPU 15 GB, got %v vCPU %v GB", obs1.Attributes.VCPU, obs1.Attributes.RAMGB)
	}
	if obs1.RegionGroup != "us-east" {
		t.Errorf("Standard2.1 expected regionGroup us-east, got %q", obs1.RegionGroup)
	}

	obs2 := obs[1]
	if obs2.Attributes.VCPU != 2 || obs2.Attributes.RAMGB != 8 {
		t.Errorf("Standard.E2.1 expected 2 vCPU 8 GB, got %v vCPU %v GB", obs2.Attributes.VCPU, obs2.Attributes.RAMGB)
	}
	if obs2.RegionGroup != "us-west" {
		t.Errorf("Standard.E2.1 expected regionGroup us-west, got %q", obs2.RegionGroup)
	}
}

func TestNormalize_QuarantineUnmappedItems(t *testing.T) {
	jsonPayload := `{
		"items": [
			{
				"partNumber": "B99999",
				"displayName": "Unknown AI Quantum Accelerator",
				"metricName": "Quantum Core Per Hour",
				"serviceCategory": "Quantum AI Service",
				"prices": [
					{
						"currencyCode": "USD",
						"prices": [{"model": "PAY_AS_YOU_GO", "value": 9.99}]
					}
				]
			},
			{
				"partNumber": "B88317",
				"displayName": "Compute - Virtual Machine - Standard2.1",
				"metricName": "OCPU Per Hour",
				"serviceCategory": "Compute - Virtual Machine",
				"prices": [
					{
						"currencyCode": "USD",
						"prices": [{"model": "PAY_AS_YOU_GO", "value": 0.0638}]
					}
				],
				"regions": ["unmapped-mars-region"]
			}
		]
	}`

	sink := &memoryQuarantineSink{}
	now := time.Now().UTC()
	obs, err := Normalize(strings.NewReader(jsonPayload), now, sink)
	if err != nil {
		t.Fatalf("Normalize failed: %v", err)
	}

	if len(obs) != 0 {
		t.Errorf("expected 0 valid observations, got %d", len(obs))
	}

	if sink.Count() != 2 {
		t.Fatalf("expected 2 quarantined items (unmapped product + unmapped region), got %d", sink.Count())
	}

	if sink.items[0].Kind != "product" || sink.items[0].RawValue != "Quantum AI Service" {
		t.Errorf("expected quarantined product, got %+v", sink.items[0])
	}
	if sink.items[1].Kind != "region" || sink.items[1].RawValue != "unmapped-mars-region" {
		t.Errorf("expected quarantined region, got %+v", sink.items[1])
	}
}

func TestNormalize_InvalidJSON(t *testing.T) {
	_, err := Normalize(bytes.NewReader([]byte("{invalid json")), time.Now().UTC())
	if err == nil {
		t.Fatal("expected error on invalid JSON, got nil")
	}
}

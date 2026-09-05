package service

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestPricingCalc_HoursInMonth(t *testing.T) {
	if !HoursInMonth.Equal(decimal.RequireFromString("730")) {
		t.Errorf("expected HoursInMonth to be 730, got %v", HoursInMonth)
	}
}

func TestPricingCalc_Storage(t *testing.T) {
	tests := []struct {
		name      string
		unitPrice string
		sizeGB    string
		monthly   string
		hourly    string
	}{
		{
			name:      "Normal usage",
			unitPrice: "0.10",
			sizeGB:    "100",
			monthly:   "10",
			hourly:    "0.01369863", // 10 / 730
		},
		{
			name:      "Zero size",
			unitPrice: "0.10",
			sizeGB:    "0",
			monthly:   "0",
			hourly:    "0",
		},
		{
			name:      "Large size",
			unitPrice: "0.023",
			sizeGB:    "10000",
			monthly:   "230",
			hourly:    "0.31506849", // 230 / 730
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unitPrice := decimal.RequireFromString(tt.unitPrice)
			sizeGB := decimal.RequireFromString(tt.sizeGB)

			expectedMonthly := decimal.RequireFromString(tt.monthly)
			expectedHourly := decimal.RequireFromString(tt.hourly)

			monthly := CalculateStorageMonthlyCost(unitPrice, sizeGB)
			if !monthly.Equal(expectedMonthly) {
				t.Errorf("monthly: expected %v, got %v", expectedMonthly, monthly)
			}

			hourly := CalculateStorageHourlyCost(unitPrice, sizeGB)
			if !hourly.Round(8).Equal(expectedHourly) {
				t.Errorf("hourly: expected %v, got %v", expectedHourly, hourly.Round(8))
			}
		})
	}
}

func TestPricingCalc_Network(t *testing.T) {
	tests := []struct {
		name      string
		unitPrice string
		egressGB  string
		monthly   string
		hourly    string
	}{
		{
			name:      "Normal usage",
			unitPrice: "0.05",
			egressGB:  "1000",
			monthly:   "50",
			hourly:    "0.06849315", // 50 / 730
		},
		{
			name:      "Zero size",
			unitPrice: "0.10",
			egressGB:  "0",
			monthly:   "0",
			hourly:    "0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unitPrice := decimal.RequireFromString(tt.unitPrice)
			egressGB := decimal.RequireFromString(tt.egressGB)
			expectedMonthly := decimal.RequireFromString(tt.monthly)
			expectedHourly := decimal.RequireFromString(tt.hourly)

			monthly := CalculateNetworkMonthlyCost(unitPrice, egressGB)
			if !monthly.Equal(expectedMonthly) {
				t.Errorf("monthly: expected %v, got %v", expectedMonthly, monthly)
			}

			hourly := CalculateNetworkHourlyCost(unitPrice, egressGB)
			if !hourly.Round(8).Equal(expectedHourly) {
				t.Errorf("hourly: expected %v, got %v", expectedHourly, hourly.Round(8))
			}
		})
	}
}

func TestPricingCalc_IOPS(t *testing.T) {
	tests := []struct {
		name      string
		unitPrice string
		iops      int64
		monthly   string
		hourly    string
	}{
		{
			name:      "Normal usage",
			unitPrice: "0.005",
			iops:      3000,
			monthly:   "15",
			hourly:    "0.02054795", // 15 / 730
		},
		{
			name:      "Zero IOPS",
			unitPrice: "0.10",
			iops:      0,
			monthly:   "0",
			hourly:    "0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unitPrice := decimal.RequireFromString(tt.unitPrice)
			expectedMonthly := decimal.RequireFromString(tt.monthly)
			expectedHourly := decimal.RequireFromString(tt.hourly)

			monthly := CalculateIOPSMonthlyCost(unitPrice, tt.iops)
			if !monthly.Equal(expectedMonthly) {
				t.Errorf("monthly: expected %v, got %v", expectedMonthly, monthly)
			}

			hourly := CalculateIOPSHourlyCost(unitPrice, tt.iops)
			if !hourly.Round(8).Equal(expectedHourly) {
				t.Errorf("hourly: expected %v, got %v", expectedHourly, hourly.Round(8))
			}
		})
	}
}

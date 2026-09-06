package service

import (
	"fmt"
	"testing"
)

func TestThresholdsForCategory(t *testing.T) {
	tests := []struct {
		category string
		expected CategoryThresholds
	}{
		{"compute", ComputeThresholds},
		{"storage", StorageThresholds},
		{"network", NetworkThresholds},
		{"database_rdbms", DatabaseRDBMSThresholds},
		{"database_nosql", DatabaseNoSQLThresholds},
		{"kubernetes", KubernetesThresholds},
		{"serverless", ServerlessThresholds},
		{"unknown", ComputeThresholds},
	}

	for _, tt := range tests {
		t.Run(tt.category, func(t *testing.T) {
			actual := ThresholdsForCategory(tt.category)
			if actual != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, actual)
			}
		})
	}
}

func TestClassifyTier(t *testing.T) {
	th := ComputeThresholds // Exact: 0.0, Close: 0.10, Approx: 0.50

	tests := []struct {
		distance float64
		expected string
	}{
		{0.0, "exact"},
		{0.05, "close"},
		{0.10, "close"},
		{0.11, "approximate"},
		{0.50, "approximate"},
		{0.51, "none"},
		{1.0, "none"},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("distance_%.2f", tt.distance), func(t *testing.T) {
			actual := classifyTier(tt.distance, th)
			if actual != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, actual)
			}
		})
	}
}

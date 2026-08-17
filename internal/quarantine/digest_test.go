package quarantine_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/thatengineerguy21/CloudVitta/internal/quarantine"
	"github.com/thatengineerguy21/CloudVitta/internal/storage"
)

func TestLevenshteinDistance(t *testing.T) {
	tests := []struct {
		s1, s2 string
		want   int
	}{
		{"", "", 0},
		{"a", "", 1},
		{"", "b", 1},
		{"kitten", "sitting", 3},
		{"eastus", "eastus2", 1},
		{"Vodafone - Dortmund", "Europe (Vodafone) - Dortmund", 9},
	}

	for _, tt := range tests {
		got := quarantine.LevenshteinDistance(tt.s1, tt.s2)
		if got != tt.want {
			t.Errorf("LevenshteinDistance(%q, %q) = %d, want %d", tt.s1, tt.s2, got, tt.want)
		}
	}
}

func TestGenerateDigest(t *testing.T) {
	now := time.Now().UTC()
	items := []quarantine.UnmappedItem{
		{
			Provider:   "aws",
			Category:   "compute",
			Kind:       "region",
			RawValue:   "Europe (Vodafone) - Dortmund",
			SkuID:      "SKU-AWS-1",
			ObservedAt: now.Add(-10 * time.Minute),
		},
		{
			Provider:   "aws",
			Category:   "compute",
			Kind:       "region",
			RawValue:   "Europe (Vodafone) - Dortmund",
			SkuID:      "SKU-AWS-2",
			ObservedAt: now,
		},
		{
			Provider:   "azure",
			Category:   "compute",
			Kind:       "region",
			RawValue:   "Intercontinental",
			SkuID:      "SKU-AZ-1",
			ObservedAt: now,
		},
	}

	report := quarantine.GenerateDigest(items)
	if report.TotalItems != 3 {
		t.Fatalf("TotalItems = %d, want 3", report.TotalItems)
	}
	if report.UniqueCount != 2 {
		t.Fatalf("UniqueCount = %d, want 2", report.UniqueCount)
	}

	// First entry should be the AWS region (count=2)
	e0 := report.Entries[0]
	if e0.RawValue != "Europe (Vodafone) - Dortmund" {
		t.Errorf("e0.RawValue = %q, want 'Europe (Vodafone) - Dortmund'", e0.RawValue)
	}
	if e0.Count != 2 {
		t.Errorf("e0.Count = %d, want 2", e0.Count)
	}
	if e0.TargetFile != "internal/matching/regionmap/aws.go" {
		t.Errorf("e0.TargetFile = %q, want 'internal/matching/regionmap/aws.go'", e0.TargetFile)
	}

	md := report.Markdown()
	if !strings.Contains(md, "Quarantine Digest Report") {
		t.Errorf("expected markdown to contain header")
	}
	if !strings.Contains(md, "Europe (Vodafone) - Dortmund") {
		t.Errorf("expected markdown to contain unmapped item")
	}

	snippets := report.GoSnippets()
	if !strings.Contains(snippets, "Europe (Vodafone) - Dortmund") {
		t.Errorf("expected snippets to contain Go map key")
	}
}

func TestGenerateDigestFromStorage(t *testing.T) {
	memStorage := storage.NewMemoryRawStorage()
	now := time.Now().UTC()

	sink := quarantine.NewStorageSink(memStorage, "gcp", "compute", "test-fetch-1", now)
	err := sink.Record(context.Background(), quarantine.UnmappedItem{
		Provider:   "gcp",
		Category:   "compute",
		Kind:       "region",
		RawValue:   "us-west8",
		SkuID:      "SKU-GCP-1",
		ObservedAt: now,
	})
	if err != nil {
		t.Fatalf("sink.Record error: %v", err)
	}
	if err := sink.Flush(context.Background()); err != nil {
		t.Fatalf("sink.Flush error: %v", err)
	}

	report, err := quarantine.GenerateDigestFromStorage(context.Background(), memStorage, "quarantine/")
	if err != nil {
		t.Fatalf("GenerateDigestFromStorage error: %v", err)
	}
	if report.TotalItems != 1 {
		t.Fatalf("TotalItems = %d, want 1", report.TotalItems)
	}
	if report.Entries[0].RawValue != "us-west8" {
		t.Errorf("Entry raw value = %q, want 'us-west8'", report.Entries[0].RawValue)
	}
}

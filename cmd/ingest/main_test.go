package main

import (
	"context"
	"testing"

	"github.com/thatengineerguy21/CloudVitta/internal/adapter/provider"
	"github.com/thatengineerguy21/CloudVitta/internal/config"
	"github.com/thatengineerguy21/CloudVitta/internal/service"
	"github.com/thatengineerguy21/CloudVitta/internal/storage"
)

func indexJobs(jobs []provider.Job) map[provider.JobKey]bool {
	registered := make(map[provider.JobKey]bool, len(jobs))
	for _, j := range jobs {
		registered[provider.JobKey{Provider: j.Provider, Category: j.Category}] = true
	}
	return registered
}

func TestFactory_DeclaredCategoriesParity(t *testing.T) {
	memStorage := storage.NewMemoryRawStorage()
	cfg := &config.Config{
		Primary: config.PrimaryConfig{
			Environment: "test",
		},
		Alibaba: config.AlibabaConfig{
			AccessKeyID:     "test-key-id",
			AccessKeySecret: "test-secret",
		},
		DigitalOcean: config.DigitalOceanConfig{
			Token: "test-token",
		},
		GCP: config.GCPConfig{
			APIKey: "test-key",
		},
	}

	factory := buildProviderFactory(cfg, memStorage, context.Background())
	registered := indexJobs(factory.BuildJobs())

	for _, p := range service.SupportedProviders() {
		categories := service.SupportedCategoriesForProvider(p)
		if len(categories) == 0 {
			t.Errorf("provider %q declares zero supported categories", p)
		}
		for _, cat := range categories {
			key := provider.JobKey{Provider: p, Category: cat}
			if !registered[key] {
				t.Errorf("missing factory registration for provider %q, category %q", p, cat)
			}
		}
	}
}

func TestFactory_ConditionalRegistrations(t *testing.T) {
	memStorage := storage.NewMemoryRawStorage()

	// Config without Alibaba credentials and DigitalOcean token
	cfg := &config.Config{
		Primary: config.PrimaryConfig{
			Environment: "test",
		},
	}

	factory := buildProviderFactory(cfg, memStorage, context.Background())
	registered := indexJobs(factory.BuildJobs())

	// Alibaba and DigitalOcean should not be registered when credentials are empty
	for key := range registered {
		if key.Provider == "alibaba" {
			t.Errorf("expected alibaba to be skipped when credentials omitted, but found category %q", key.Category)
		}
		if key.Provider == "digitalocean" {
			t.Errorf("expected digitalocean to be skipped when token omitted, but found category %q", key.Category)
		}
	}

	// Core providers (AWS, Azure, GCP, Oracle, IBM) should still be registered.
	// IBM registers adapters unconditionally; authentication failure occurs at ingestion
	// runtime, not at factory registration time (asymmetric to Alibaba/DigitalOcean).
	coreProviders := []string{"aws", "azure", "gcp", "oracle", "ibm"}
	for _, p := range coreProviders {
		for _, cat := range service.SupportedCategoriesForProvider(p) {
			key := provider.JobKey{Provider: p, Category: cat}
			if !registered[key] {
				t.Errorf("missing factory registration for core provider %q, category %q", p, cat)
			}
		}
	}
}

func TestFactory_MultiRegionClientConfiguration(t *testing.T) {
	memStorage := storage.NewMemoryRawStorage()
	cfg := &config.Config{
		Primary: config.PrimaryConfig{
			Environment: "test",
		},
	}

	factory := buildProviderFactory(cfg, memStorage, context.Background())
	jobs := factory.BuildJobs()

	providerCounts := make(map[string]int)
	for _, j := range jobs {
		if j.Provider != "aws" && j.Provider != "azure" {
			continue
		}
		providerCounts[j.Provider]++

		checker, ok := j.Adapter.(interface{ HasCustomURL() bool })
		if !ok {
			t.Fatalf("%s adapter for category %s does not implement HasCustomURL()", j.Provider, j.Category)
		}

		if j.Category == "network" {
			if !checker.HasCustomURL() {
				t.Errorf("%s network client should have custom URL set to global offer/meter", j.Provider)
			}
		} else {
			if checker.HasCustomURL() {
				t.Errorf("%s %s client should not have custom URL (must support multi-region)", j.Provider, j.Category)
			}
		}
	}

	if providerCounts["aws"] != 7 {
		t.Errorf("expected 7 AWS jobs, got %d", providerCounts["aws"])
	}
	if providerCounts["azure"] != 7 {
		t.Errorf("expected 7 Azure jobs, got %d", providerCounts["azure"])
	}
}

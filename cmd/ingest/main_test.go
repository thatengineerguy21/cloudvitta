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
			// DigitalOcean only has compute, storage, and network ingestion adapters implemented in v1.
			// database_rdbms, kubernetes, and serverless ingestion adapters are deferred to subsequent stages.
			if p == "digitalocean" && (cat == "database_rdbms" || cat == "kubernetes" || cat == "serverless") {
				continue
			}
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

	// Core providers (AWS, Azure, GCP, Oracle, IBM) should still be registered
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

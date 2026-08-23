package service

import (
	"github.com/thatengineerguy21/CloudVitta/internal/adapter/provider/aws"
	"github.com/thatengineerguy21/CloudVitta/internal/adapter/provider/azure"
	"github.com/thatengineerguy21/CloudVitta/internal/adapter/provider/gcp"
)

// IsProviderCategorySupported checks if a provider officially declares support for the category.
// As defined in ADR 0026 and PRD §8.1, a provider not declaring a category must never enter matching for it.
func IsProviderCategorySupported(provider, category string) bool {
	switch provider {
	case "aws":
		return aws.IsCategorySupported(category)
	case "azure":
		return azure.IsCategorySupported(category)
	case "gcp":
		return gcp.IsCategorySupported(category)
	default:
		return false
	}
}

// SupportedProviders returns a list of actively supported cloud providers.
func SupportedProviders() []string {
	return []string{"aws", "azure", "gcp"}
}

// SupportedCategoriesForProvider returns a slice of category names supported by the provider in canonical order.
func SupportedCategoriesForProvider(provider string) []string {
	switch provider {
	case "aws", "azure", "gcp":
		return []string{"compute", "storage", "network", "database_rdbms", "database_nosql", "kubernetes"}
	default:
		return nil
	}
}

package cache

import "fmt"

// SchemaVersion defines the default Redis key schema version.
const SchemaVersion = "v1"

// SchemaVersionNetwork defines the cache key schema version for the network category.
// Bumped to v2 following network transfer taxonomy extension (ADR 0048).
const SchemaVersionNetwork = "v2"

// CategorySchemaVersion returns the appropriate Redis schema version for a service category.
func CategorySchemaVersion(category string) string {
	if category == "network" {
		return SchemaVersionNetwork
	}
	return SchemaVersion
}

// BuildKey constructs a schema-versioned Redis key for a provider, category, and region.
// Format: {schema_version}:{provider}:{category}:{region} (e.g. v1:aws:compute:us-east-1).
func BuildKey(schemaVersion, provider, category, region string) string {
	return fmt.Sprintf("%s:%s:%s:%s", schemaVersion, provider, category, region)
}

// BuildCatalogSummaryKey constructs a schema-versioned Redis key for compute instance catalog summary.
// Format: {schema_version}:catalog:summary (e.g. v1:catalog:summary).
func BuildCatalogSummaryKey(schemaVersion string) string {
	return fmt.Sprintf("%s:catalog:summary", schemaVersion)
}

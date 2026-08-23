package databaseenginemap

import (
	"errors"
	"fmt"
	"strings"
)

// Canonical database engine taxonomy constants.
const (
	EnginePostgreSQL = "postgresql"
	EngineMySQL      = "mysql"
	EngineSQLServer  = "sqlserver"
	EngineMariaDB    = "mariadb"
	EngineOracle     = "oracle"
)

// ErrUnmappedDatabaseEngine is returned when a raw engine value cannot be mapped to canonical taxonomy.
var ErrUnmappedDatabaseEngine = errors.New("databaseenginemap: unmapped database engine")

// SupportedCanonicalEngines returns the list of canonical engines active in the current stage.
func SupportedCanonicalEngines() []string {
	return []string{EnginePostgreSQL, EngineMySQL, EngineSQLServer}
}

// IsSupportedStageEngine returns true if the engine is active in the current stage (PostgreSQL, MySQL, SQLServer).
func IsSupportedStageEngine(engine string) bool {
	switch strings.ToLower(strings.TrimSpace(engine)) {
	case EnginePostgreSQL, EngineMySQL, EngineSQLServer:
		return true
	default:
		return false
	}
}

// ResolveCanonicalEngine maps a user-supplied engine string to a canonical database engine.
// It checks canonical constants first, then tries provider-specific mappings across AWS, Azure, and GCP.
// If rawEngine cannot be mapped, ErrUnmappedDatabaseEngine is returned.
func ResolveCanonicalEngine(rawEngine string) (string, error) {
	trimmed := strings.TrimSpace(rawEngine)
	if trimmed == "" {
		return "", nil
	}

	lower := strings.ToLower(trimmed)
	switch lower {
	case EnginePostgreSQL, EngineMySQL, EngineSQLServer, EngineMariaDB, EngineOracle:
		return lower, nil
	}

	for _, prov := range []string{"aws", "azure", "gcp"} {
		if norm, err := NormalizeEngine(prov, trimmed); err == nil {
			return norm, nil
		}
	}

	return "", fmt.Errorf("%w: %q", ErrUnmappedDatabaseEngine, rawEngine)
}

// NormalizeEngine resolves a provider-specific raw engine name to a canonical database engine.
func NormalizeEngine(provider, rawEngine string) (string, error) {
	switch provider {
	case "aws":
		return MapAWSEngine(rawEngine)
	case "azure":
		return MapAzureEngine(rawEngine)
	case "gcp":
		return MapGCPEngine(rawEngine)
	default:
		return "", fmt.Errorf("databaseenginemap: unmapped provider %q", provider)
	}
}

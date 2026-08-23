package databaseenginemap

import (
	"errors"
	"fmt"
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

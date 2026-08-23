package databaseenginemap

import (
	"fmt"
	"strings"
)

var gcpEngineMap = map[string]string{
	"postgresql":                EnginePostgreSQL,
	"postgres":                  EnginePostgreSQL,
	"cloud sql for postgresql":  EnginePostgreSQL,
	"alloydb for postgresql":    EnginePostgreSQL,
	"alloydb":                   EnginePostgreSQL,
	"postgres_15":               EnginePostgreSQL,
	"postgres_14":               EnginePostgreSQL,
	"postgres_13":               EnginePostgreSQL,
	"postgres_12":               EnginePostgreSQL,
	"postgres_11":               EnginePostgreSQL,
	"postgres_9_6":              EnginePostgreSQL,
	"mysql":                     EngineMySQL,
	"cloud sql for mysql":       EngineMySQL,
	"mysql_8_0":                 EngineMySQL,
	"mysql_5_7":                 EngineMySQL,
	"mysql_5_6":                 EngineMySQL,
	"sql server":                EngineSQLServer,
	"sqlserver":                 EngineSQLServer,
	"sql-server":                EngineSQLServer,
	"cloud sql for sql server":  EngineSQLServer,
	"sqlserver_2019_standard":   EngineSQLServer,
	"sqlserver_2019_enterprise": EngineSQLServer,
	"sqlserver_2019_express":    EngineSQLServer,
	"sqlserver_2019_web":        EngineSQLServer,
}

// MapGCPEngine maps a GCP database engine/service string to a canonical engine identifier.
func MapGCPEngine(rawEngine string) (string, error) {
	key := strings.ToLower(strings.TrimSpace(rawEngine))
	canonical, ok := gcpEngineMap[key]
	if !ok {
		return "", fmt.Errorf("%w: gcp engine %q", ErrUnmappedDatabaseEngine, rawEngine)
	}
	return canonical, nil
}

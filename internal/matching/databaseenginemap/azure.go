package databaseenginemap

import (
	"fmt"
	"strings"
)

var azureEngineMap = map[string]string{
	"postgresql":                    EnginePostgreSQL,
	"postgres":                      EnginePostgreSQL,
	"azure database for postgresql": EnginePostgreSQL,
	"postgresql flexible server":    EnginePostgreSQL,
	"azure database for postgresql flexible server": EnginePostgreSQL,
	"mysql":                    EngineMySQL,
	"azure database for mysql": EngineMySQL,
	"mysql flexible server":    EngineMySQL,
	"azure database for mysql flexible server": EngineMySQL,
	"sql server":                 EngineSQLServer,
	"sqlserver":                  EngineSQLServer,
	"sql-server":                 EngineSQLServer,
	"sql database":               EngineSQLServer,
	"azure sql database":         EngineSQLServer,
	"azure sql":                  EngineSQLServer,
	"general purpose":            EngineSQLServer,
	"business critical":          EngineSQLServer,
	"mariadb":                    EngineMariaDB,
	"azure database for mariadb": EngineMariaDB,
}

// MapAzureEngine maps an Azure database engine/service string to a canonical engine identifier.
func MapAzureEngine(rawEngine string) (string, error) {
	key := strings.ToLower(strings.TrimSpace(rawEngine))
	canonical, ok := azureEngineMap[key]
	if !ok {
		return "", fmt.Errorf("%w: azure engine %q", ErrUnmappedDatabaseEngine, rawEngine)
	}
	return canonical, nil
}

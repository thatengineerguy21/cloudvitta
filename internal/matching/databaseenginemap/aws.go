package databaseenginemap

import (
	"fmt"
	"strings"
)

var awsEngineMap = map[string]string{
	"postgresql":                 EnginePostgreSQL,
	"postgres":                   EnginePostgreSQL,
	"aurora-postgresql":          EnginePostgreSQL,
	"aurora postgresql":          EnginePostgreSQL,
	"amazon aurora postgresql":   EnginePostgreSQL,
	"mysql":                      EngineMySQL,
	"aurora-mysql":               EngineMySQL,
	"aurora mysql":               EngineMySQL,
	"amazon aurora mysql":        EngineMySQL,
	"sql server":                 EngineSQLServer,
	"sqlserver":                  EngineSQLServer,
	"sql-server":                 EngineSQLServer,
	"sql server enterprise":      EngineSQLServer,
	"sql server standard":        EngineSQLServer,
	"sql server web":             EngineSQLServer,
	"sql server express":         EngineSQLServer,
	"mariadb":                    EngineMariaDB,
	"oracle":                     EngineOracle,
	"oracle ee":                  EngineOracle,
	"oracle se2":                 EngineOracle,
	"oracle se1":                 EngineOracle,
	"oracle se":                  EngineOracle,
}

// MapAWSEngine maps an AWS database engine string to a canonical engine identifier.
func MapAWSEngine(rawEngine string) (string, error) {
	key := strings.ToLower(strings.TrimSpace(rawEngine))
	canonical, ok := awsEngineMap[key]
	if !ok {
		return "", fmt.Errorf("%w: aws engine %q", ErrUnmappedDatabaseEngine, rawEngine)
	}
	return canonical, nil
}

package catalogmap

import (
	"fmt"
)

var azureCatalogMap = map[string]string{
	"Virtual Machines":              "compute",
	"Storage":                       "storage",
	"Bandwidth":                     "network",
	"Azure Database for PostgreSQL": "database_rdbms",
	"Azure Database for MySQL":      "database_rdbms",
	"SQL Database":                  "database_rdbms",
	"Azure Database for MariaDB":    "database_rdbms",
	"Azure Cosmos DB":               "database_nosql",
	"Cosmos DB":                     "database_nosql",
	"Azure Kubernetes Service":      "kubernetes",
	"Functions":                     "serverless",
	"Azure Functions":               "serverless",
	"Flex Consumption":              "serverless",
}

// MapAzureProduct resolves an Azure service name to a normalized service category.
// It fails loudly with ErrUnmappedProduct if the service name is not explicitly mapped.
func MapAzureProduct(serviceName string) (string, error) {
	category, ok := azureCatalogMap[serviceName]
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUnmappedProduct, serviceName)
	}
	return category, nil
}

// KnownAzureProducts returns a copy of known Azure product mappings.
func KnownAzureProducts() map[string]string {
	m := make(map[string]string, len(azureCatalogMap))
	for k, v := range azureCatalogMap {
		m[k] = v
	}
	return m
}

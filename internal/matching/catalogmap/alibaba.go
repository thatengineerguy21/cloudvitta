package catalogmap

import (
	"fmt"
)

var alibabaCatalogMap = map[string]string{
	"ecs":                              "compute",
	"ecs.instance":                     "compute",
	"Virtual Machine":                  "compute",
	"Elastic Compute Service":          "compute",
	"oss":                              "storage",
	"Object Storage Service":           "storage",
	"disk":                             "storage",
	"ebs":                              "storage",
	"Block Storage":                    "storage",
	"data_transfer":                    "network",
	"eip":                              "network",
	"Elastic IP":                       "network",
	"rds":                              "database_rdbms",
	"ApsaraDB RDS":                     "database_rdbms",
	"polardb":                          "database_rdbms",
	"polardb_mysql":                    "database_rdbms",
	"polardb_pg":                       "database_rdbms",
	"lindorm":                          "database_nosql",
	"mongodb":                          "database_nosql",
	"ack":                              "kubernetes",
	"Container Service for Kubernetes": "kubernetes",
	"fc":                               "serverless",
	"Function Compute":                 "serverless",
}

// MapAlibabaProduct resolves an Alibaba Cloud product or service category code to a normalized service category.
// It fails loudly with ErrUnmappedProduct if the product code is not explicitly mapped.
func MapAlibabaProduct(productCode string) (string, error) {
	category, ok := alibabaCatalogMap[productCode]
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUnmappedProduct, productCode)
	}
	return category, nil
}

// KnownAlibabaProducts returns a copy of known Alibaba Cloud product mappings.
func KnownAlibabaProducts() map[string]string {
	m := make(map[string]string, len(alibabaCatalogMap))
	for k, v := range alibabaCatalogMap {
		m[k] = v
	}
	return m
}

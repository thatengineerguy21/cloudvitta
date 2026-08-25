package catalogmap

import (
	"fmt"
)

var oracleCatalogMap = map[string]string{
	"Compute - Virtual Machine":                    "compute",
	"Compute":                                      "compute",
	"Virtual Machine":                              "compute",
	"Storage - Block Volume":                       "storage",
	"Storage - Block Volumes":                      "storage",
	"Storage - Object Storage":                     "storage",
	"Storage - File Storage":                       "storage",
	"Networking - Virtual Cloud Network":           "network",
	"Networking - Virtual Cloud Networks":          "network",
	"Database":                                     "database_rdbms",
	"Database - Cloud Service":                     "database_rdbms",
	"Container Engine for Kubernetes":              "kubernetes",
	"Cloud Infrastructure Kubernetes Engine (OKE)": "kubernetes",
	"Oracle Container Engine for Kubernetes":       "kubernetes",
	"Functions":                                    "serverless",
}

// MapOracleProduct resolves an Oracle OCI product or service category code to a normalized service category.
// It fails loudly with ErrUnmappedProduct if the product code is not explicitly mapped.
func MapOracleProduct(productCode string) (string, error) {
	category, ok := oracleCatalogMap[productCode]
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUnmappedProduct, productCode)
	}
	return category, nil
}

// KnownOracleProducts returns a copy of known Oracle product mappings.
func KnownOracleProducts() map[string]string {
	m := make(map[string]string, len(oracleCatalogMap))
	for k, v := range oracleCatalogMap {
		m[k] = v
	}
	return m
}

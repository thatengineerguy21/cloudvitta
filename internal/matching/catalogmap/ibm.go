package catalogmap

import (
	"fmt"
)

var ibmCatalogMap = map[string]string{
	"is.instance":              "compute",
	"virtual-server-for-vpc":   "compute",
	"is.volume":                "storage",
	"cloud-object-storage":     "storage",
	"is.floating-ip":           "network",
	"is.public-gateway":        "network",
	"databases-for-postgresql": "database_rdbms",
	"databases-for-mysql":      "database_rdbms",
	"databases-for-mongodb":    "database_nosql",
	"containers-kubernetes":    "kubernetes",
	"code-engine":              "serverless",
}

// MapIBMProduct resolves an IBM Cloud product or service category code to a normalized service category.
// It fails loudly with ErrUnmappedProduct if the product code is not explicitly mapped.
func MapIBMProduct(productCode string) (string, error) {
	category, ok := ibmCatalogMap[productCode]
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUnmappedProduct, productCode)
	}
	return category, nil
}

// KnownIBMProducts returns a copy of known IBM Cloud product mappings.
func KnownIBMProducts() map[string]string {
	m := make(map[string]string, len(ibmCatalogMap))
	for k, v := range ibmCatalogMap {
		m[k] = v
	}
	return m
}

package catalogmap

import (
	"fmt"
)

var digitalOceanCatalogMap = map[string]string{
	"droplet":           "compute",
	"droplets":          "compute",
	"Droplet":           "compute",
	"Droplets":          "compute",
	"spaces":            "storage",
	"Spaces":            "storage",
	"volume":            "storage",
	"volumes":           "storage",
	"Volume":            "storage",
	"Volumes":           "storage",
	"bandwidth":         "network",
	"Bandwidth":         "network",
	"data_transfer":     "network",
	"database":          "database_rdbms",
	"database_rdbms":    "database_rdbms",
	"dbaas":             "database_rdbms",
	"Managed Databases": "database_rdbms",
	"kubernetes":        "kubernetes",
	"doks":              "kubernetes",
	"Kubernetes":        "kubernetes",
	"serverless":        "serverless",
	"functions":         "serverless",
	"Functions":         "serverless",
}

// MapDigitalOceanProduct resolves a DigitalOcean product or service category code to a normalized service category.
// It fails loudly with ErrUnmappedProduct if the product code is not explicitly mapped.
func MapDigitalOceanProduct(productCode string) (string, error) {
	category, ok := digitalOceanCatalogMap[productCode]
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUnmappedProduct, productCode)
	}
	return category, nil
}

// KnownDigitalOceanProducts returns a copy of known DigitalOcean product mappings.
func KnownDigitalOceanProducts() map[string]string {
	m := make(map[string]string, len(digitalOceanCatalogMap))
	for k, v := range digitalOceanCatalogMap {
		m[k] = v
	}
	return m
}

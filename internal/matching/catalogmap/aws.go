package catalogmap

import (
	"errors"
	"fmt"
)

// ErrUnmappedProduct is returned when an AWS product code has no entry in the catalog map.
var ErrUnmappedProduct = errors.New("catalogmap: unmapped product code")

var awsCatalogMap = map[string]string{
	"AmazonEC2":        "compute",
	"AmazonS3":         "storage",
	"AWSDataTransfer":  "network",
	"AmazonRDS":        "database_rdbms",
	"AmazonRDSStorage": "database_rdbms",
	"AmazonAurora":     "database_rdbms",
	"AmazonDynamoDB":   "database_nosql",
}

// MapAWSProduct resolves an AWS product code to a normalized service category.
// It fails loudly with ErrUnmappedProduct if the product code is not explicitly mapped.
func MapAWSProduct(productCode string) (string, error) {
	category, ok := awsCatalogMap[productCode]
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUnmappedProduct, productCode)
	}
	return category, nil
}

// KnownAWSProducts returns a copy of known AWS product mappings.
func KnownAWSProducts() map[string]string {
	m := make(map[string]string, len(awsCatalogMap))
	for k, v := range awsCatalogMap {
		m[k] = v
	}
	return m
}

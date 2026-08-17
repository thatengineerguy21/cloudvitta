package storageclassmap

import (
	"fmt"
)

var gcpStorageClassMap = map[string]string{
	"Standard":       "standard",
	"Regional":       "standard",
	"Multi-Regional": "standard",
	"Dual-Region":    "standard",
	"Nearline":       "infrequent_access",
	"Coldline":       "archive",
	"Archive":        "archive",
	"DRAStorage":     "infrequent_access",
	"DRA":            "infrequent_access",
}

// MapGCPStorageClass resolves a GCP storage class to a canonical class.
func MapGCPStorageClass(rawClass string) (string, error) {
	canonical, ok := gcpStorageClassMap[rawClass]
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUnmappedStorageClass, rawClass)
	}
	return canonical, nil
}

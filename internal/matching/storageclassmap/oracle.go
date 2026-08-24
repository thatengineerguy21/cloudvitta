package storageclassmap

import (
	"fmt"
)

var oracleStorageClassMap = map[string]string{
	// Object Storage
	"standard":                                   "standard",
	"Standard":                                   "standard",
	"Object Storage Standard":                    "standard",
	"Object Storage - Storage":                   "standard",
	"Storage - Object Storage":                   "standard",
	"infrequent_access":                          "infrequent_access",
	"Infrequent Access":                          "infrequent_access",
	"Object Storage Infrequent Access":           "infrequent_access",
	"Object Storage - Infrequent Access":         "infrequent_access",
	"Object Storage - Infrequent Access Storage": "infrequent_access",
	"archive":                          "archive",
	"Archive":                          "archive",
	"Archive Storage":                  "archive",
	"Object Storage Archive":           "archive",
	"Object Storage - Archive":         "archive",
	"Object Storage - Archive Storage": "archive",

	// Block Storage / Block Volumes
	"Block Volume":                      "standard",
	"Block Volumes":                     "standard",
	"Block Volume - Storage":            "standard",
	"Block Volume Standard":             "standard",
	"Block Volume - Balanced":           "standard",
	"Block Volume - Higher Performance": "standard",
	"Block Volume - Lower Cost":         "standard",
}

// MapOracleStorageClass resolves an Oracle OCI storage class to a canonical class.
func MapOracleStorageClass(rawClass string) (string, error) {
	canonical, ok := oracleStorageClassMap[rawClass]
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUnmappedStorageClass, rawClass)
	}
	return canonical, nil
}

// KnownOracleStorageClasses returns a copy of known Oracle storage class mappings.
func KnownOracleStorageClasses() map[string]string {
	m := make(map[string]string, len(oracleStorageClassMap))
	for k, v := range oracleStorageClassMap {
		m[k] = v
	}
	return m
}

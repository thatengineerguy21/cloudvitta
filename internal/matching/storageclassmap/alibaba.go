package storageclassmap

import (
	"fmt"
)

var alibabaStorageClassMap = map[string]string{
	// Object Storage Service (OSS)
	"standard":          "standard",
	"Standard":          "standard",
	"oss-standard":      "standard",
	"Standard (LRS)":    "standard",
	"Standard (ZRS)":    "standard",
	"Standard_LRS":      "standard",
	"Standard_ZRS":      "standard",
	"ia":                "infrequent_access",
	"IA":                "infrequent_access",
	"oss-ia":            "infrequent_access",
	"Infrequent Access": "infrequent_access",
	"infrequent_access": "infrequent_access",
	"IA (LRS)":          "infrequent_access",
	"IA (ZRS)":          "infrequent_access",
	"IA_LRS":            "infrequent_access",
	"IA_ZRS":            "infrequent_access",
	"archive":           "archive",
	"Archive":           "archive",
	"oss-archive":       "archive",
	"Archive (LRS)":     "archive",
	"Archive_LRS":       "archive",
	"Cold Archive":      "archive",
	"ColdArchive":       "archive",
	"cold_archive":      "archive",

	// Block Storage (Disk / EBS)
	"cloud_essd":       "standard",
	"cloud_ssd":        "standard",
	"cloud_efficiency": "standard",
	"cloud":            "standard",
	"san_ssd":          "standard",
	"san_efficiency":   "standard",
	"essd":             "standard",
	"ssd":              "standard",
	"efficiency":       "standard",
}

// MapAlibabaStorageClass resolves an Alibaba Cloud storage class to a canonical class.
func MapAlibabaStorageClass(rawClass string) (string, error) {
	canonical, ok := alibabaStorageClassMap[rawClass]
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUnmappedStorageClass, rawClass)
	}
	return canonical, nil
}

// KnownAlibabaStorageClasses returns a copy of known Alibaba Cloud storage class mappings.
func KnownAlibabaStorageClasses() map[string]string {
	m := make(map[string]string, len(alibabaStorageClassMap))
	for k, v := range alibabaStorageClassMap {
		m[k] = v
	}
	return m
}

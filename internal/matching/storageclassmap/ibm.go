package storageclassmap

import (
	"fmt"
)

var ibmStorageClassMap = map[string]string{
	// Cloud Object Storage (COS)
	"standard":           "standard",
	"Standard":           "standard",
	"COS Standard":       "standard",
	"standard-storage":   "standard",
	"Smart Tier":         "standard",
	"smart-tier":         "standard",
	"vault":              "infrequent_access",
	"Vault":              "infrequent_access",
	"COS Vault":          "infrequent_access",
	"vault-storage":      "infrequent_access",
	"infrequent_access":  "infrequent_access",
	"cold_vault":         "archive",
	"Cold Vault":         "archive",
	"COS Cold Vault":     "archive",
	"cold-vault-storage": "archive",
	"archive":            "archive",
	"Archive":            "archive",
	"cold":               "archive",

	// Block Storage (is.volume)
	"is.volume":               "standard",
	"general-purpose":         "standard",
	"general-purpose-storage": "standard",
	"5iops-tier":              "standard",
	"tier-5iops":              "standard",
	"tier-5iops-storage":      "standard",
	"10iops-tier":             "standard",
	"tier-10iops":             "standard",
	"tier-10iops-storage":     "standard",
	"custom":                  "standard",
	"custom-storage":          "standard",
}

// MapIBMStorageClass resolves an IBM Cloud storage class to a canonical class.
func MapIBMStorageClass(rawClass string) (string, error) {
	canonical, ok := ibmStorageClassMap[rawClass]
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUnmappedStorageClass, rawClass)
	}
	return canonical, nil
}

// KnownIBMStorageClasses returns a copy of known IBM Cloud storage class mappings.
func KnownIBMStorageClasses() map[string]string {
	m := make(map[string]string, len(ibmStorageClassMap))
	for k, v := range ibmStorageClassMap {
		m[k] = v
	}
	return m
}

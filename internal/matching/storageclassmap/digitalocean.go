package storageclassmap

import (
	"fmt"
)

var digitalOceanStorageClassMap = map[string]string{
	// Spaces Object Storage
	"spaces":                "standard",
	"Spaces":                "standard",
	"Spaces Object Storage": "standard",
	"spaces-storage":        "standard",
	"standard":              "standard",
	"Standard":              "standard",

	// Volumes Block Storage
	"volume":               "standard",
	"volumes":              "standard",
	"Volume":               "standard",
	"Volumes":              "standard",
	"volume-storage":       "standard",
	"Block Storage":        "standard",
	"Block Storage Volume": "standard",
	"Block Volume":         "standard",
}

// MapDigitalOceanStorageClass resolves a DigitalOcean storage class to a canonical class.
func MapDigitalOceanStorageClass(rawClass string) (string, error) {
	canonical, ok := digitalOceanStorageClassMap[rawClass]
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUnmappedStorageClass, rawClass)
	}
	return canonical, nil
}

// KnownDigitalOceanStorageClasses returns a copy of known DigitalOcean storage class mappings.
func KnownDigitalOceanStorageClasses() map[string]string {
	m := make(map[string]string, len(digitalOceanStorageClassMap))
	for k, v := range digitalOceanStorageClassMap {
		m[k] = v
	}
	return m
}

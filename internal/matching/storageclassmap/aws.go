package storageclassmap

import (
	"errors"
	"fmt"
)

// ErrUnmappedStorageClass is returned when a provider storage class has no entry in the map.
var ErrUnmappedStorageClass = errors.New("storageclassmap: unmapped storage class")

var awsStorageClassMap = map[string]string{
	// S3 Storage Classes
	"Standard":                     "standard",
	"General Purpose":              "standard",
	"Intelligent-Tiering":          "standard",
	"Standard - Infrequent Access": "infrequent_access",
	"Standard-IA":                  "infrequent_access",
	"One Zone - Infrequent Access": "infrequent_access",
	"OneZone-IA":                   "infrequent_access",
	"Glacier Instant Retrieval":    "archive",
	"Glacier Flexible Retrieval":   "archive",
	"Glacier":                      "archive",
	"Glacier Deep Archive":         "archive",
	"Archive":                      "archive",
	"Express One Zone":             "standard",
	"Reduced Redundancy":           "standard",
	"S3 Outposts":                  "standard",

	// EBS Volume Types (volumeType)
	"gp2":                       "standard",
	"gp3":                       "standard",
	"io1":                       "standard",
	"io2":                       "standard",
	"st1":                       "standard",
	"sc1":                       "archive",
	"Magnetic":                  "standard",
	"Throughput Optimized HDD":  "standard",
	"Cold HDD":                  "archive",
	"Provisioned IOPS":          "standard",
	"Provisioned IOPS SSD(io1)": "standard",
	"Provisioned IOPS SSD(io2)": "standard",
}

// MapAWSStorageClass resolves an AWS storage class to a canonical class.
func MapAWSStorageClass(rawClass string) (string, error) {
	canonical, ok := awsStorageClassMap[rawClass]
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUnmappedStorageClass, rawClass)
	}
	return canonical, nil
}

// KnownAWSStorageClasses returns a copy of known AWS storage class mappings.
func KnownAWSStorageClasses() map[string]string {
	m := make(map[string]string, len(awsStorageClassMap))
	for k, v := range awsStorageClassMap {
		m[k] = v
	}
	return m
}

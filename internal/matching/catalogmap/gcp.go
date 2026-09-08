package catalogmap

import (
	"fmt"
)

var gcpCatalogMap = map[string]string{
	"Compute Engine":           "compute",
	"6F81-5844-456A":           "compute",
	"Cloud Storage":            "storage",
	"95FF-2EF5-5EA1":           "storage",
	"Cloud Interconnect":       "network",
	"Networking":               "network",
	"Bandwidth":                "network",
	"E89B-A08C-8A2D":           "network",
	"Cloud SQL":                "database_rdbms",
	"AlloyDB":                  "database_rdbms",
	"9662-B51E-5089":           "database_rdbms",
	"AlloyDB for PostgreSQL":   "database_rdbms",
	"C49F-B7F2-7416":           "database_rdbms",
	"Cloud Firestore":          "database_nosql",
	"Firestore":                "database_nosql",
	"Cloud Datastore":          "database_nosql",
	"Datastore":                "database_nosql",
	"EE2C-7FAC-5E08":           "database_nosql",
	"E24D-7981-67BD":           "database_nosql",
	"C237-7D12-9F12":           "database_nosql",
	"Cloud Bigtable":           "database_nosql",
	"Bigtable":                 "database_nosql",
	"C802-861C-2155":           "database_nosql",
	"Kubernetes Engine":        "kubernetes",
	"Google Kubernetes Engine": "kubernetes",
	"CCD8-9BF1-090E":           "kubernetes",
	"24E6-581D-38E5":           "kubernetes",
	"44CD-3C5E-2A4B":           "kubernetes",
	"Cloud Functions":          "serverless",
	"Cloud Run functions":      "serverless",
	"Cloud Run":                "serverless",
	"29E7-DA93-CA13":           "serverless",
	"152E-C115-5142":           "serverless",
}

// MapGCPProduct resolves a GCP service display name or service ID to a normalized service category.
// It fails loudly with ErrUnmappedProduct if the service is not explicitly mapped.
func MapGCPProduct(serviceName string) (string, error) {
	category, ok := gcpCatalogMap[serviceName]
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUnmappedProduct, serviceName)
	}
	return category, nil
}

// KnownGCPProducts returns a copy of known GCP product mappings.
func KnownGCPProducts() map[string]string {
	m := make(map[string]string, len(gcpCatalogMap))
	for k, v := range gcpCatalogMap {
		m[k] = v
	}
	return m
}

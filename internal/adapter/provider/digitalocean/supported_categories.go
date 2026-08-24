package digitalocean

// supportedCategories maps service categories supported by the DigitalOcean provider adapter.
// Open Question #7 Resolution: DigitalOcean supports compute, storage, network, database_rdbms,
// kubernetes, and serverless, but explicitly omits database_nosql.
var supportedCategories = map[string]bool{
	"compute":        true,
	"storage":        true,
	"network":        true,
	"database_rdbms": true,
	"kubernetes":     true,
	"serverless":     true,
	// database_nosql is intentionally omitted (Open Question #7)
}

// IsCategorySupported reports whether the specified category is supported by the DigitalOcean adapter.
func IsCategorySupported(category string) bool {
	return supportedCategories[category]
}

// SupportedCategories returns a copy of the supported category names for DigitalOcean.
func SupportedCategories() []string {
	return []string{"compute", "storage", "network", "database_rdbms", "kubernetes", "serverless"}
}

package azure

// supportedCategories maps service categories supported by the Azure provider adapter.
var supportedCategories = map[string]bool{
	"compute":        true,
	"storage":        true,
	"network":        true,
	"database_rdbms": true,
	"database_nosql": true,
	"kubernetes":     true,
	"serverless":     true,
}

// IsCategorySupported reports whether the specified category is supported by the Azure adapter.
func IsCategorySupported(category string) bool {
	return supportedCategories[category]
}

// SupportedCategories returns a copy of the supported category names.
func SupportedCategories() []string {
	return []string{"compute", "storage", "network", "database_rdbms", "database_nosql", "kubernetes", "serverless"}
}

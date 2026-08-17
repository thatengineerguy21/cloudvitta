package gcp

// supportedCategories maps service categories supported by the GCP provider adapter.
var supportedCategories = map[string]bool{
	"compute": true,
	"storage": true,
	"network": true,
}

// IsCategorySupported reports whether the specified category is supported by the GCP adapter.
func IsCategorySupported(category string) bool {
	return supportedCategories[category]
}

// SupportedCategories returns a copy of the supported category names.
func SupportedCategories() []string {
	return []string{"compute", "storage", "network"}
}

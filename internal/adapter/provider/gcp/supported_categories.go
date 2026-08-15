package gcp

// SupportedCategories maps service categories supported by the GCP provider adapter.
var SupportedCategories = map[string]bool{
	"compute": true,
	"storage": true,
}

// IsCategorySupported reports whether the specified category is supported by the GCP adapter.
func IsCategorySupported(category string) bool {
	return SupportedCategories[category]
}

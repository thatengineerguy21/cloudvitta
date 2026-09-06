package digitalocean

// supportedCategories maps service categories supported by the DigitalOcean provider adapter.
// Open Question #7 Resolution: DigitalOcean supports compute, storage, and network.
// database_rdbms, kubernetes, serverless, and database_nosql are not supported because
// DigitalOcean does not expose public pricing APIs for these categories.
var supportedCategories = map[string]bool{
	"compute": true,
	"storage": true,
	"network": true,
}

// IsCategorySupported reports whether the specified category is supported by the DigitalOcean adapter.
func IsCategorySupported(category string) bool {
	return supportedCategories[category]
}

// SupportedCategories returns a copy of the supported category names for DigitalOcean.
func SupportedCategories() []string {
	return []string{"compute", "storage", "network"}
}

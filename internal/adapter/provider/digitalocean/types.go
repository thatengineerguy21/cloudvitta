package digitalocean

// Size represents a Droplet size definition returned by DigitalOcean API v2.
type Size struct {
	Slug         string   `json:"slug"`
	Memory       int      `json:"memory"`
	VCPUs        int      `json:"vcpus"`
	Disk         int      `json:"disk"`
	Transfer     float64  `json:"transfer"`
	PriceMonthly float64  `json:"price_monthly"`
	PriceHourly  float64  `json:"price_hourly"`
	Regions      []string `json:"regions"`
	Available    bool     `json:"available"`
	Description  string   `json:"description,omitempty"`
}

// SizesResponse represents the top-level payload returned by GET /v2/sizes.
type SizesResponse struct {
	Sizes []Size `json:"sizes"`
	Links *Links `json:"links,omitempty"`
	Meta  *Meta  `json:"meta,omitempty"`
}

// Region represents a region definition returned by DigitalOcean API v2.
type Region struct {
	Slug      string   `json:"slug"`
	Name      string   `json:"name"`
	Sizes     []string `json:"sizes,omitempty"`
	Available bool     `json:"available"`
}

// RegionsResponse represents the top-level payload returned by GET /v2/regions.
type RegionsResponse struct {
	Regions []Region `json:"regions"`
	Links   *Links   `json:"links,omitempty"`
	Meta    *Meta    `json:"meta,omitempty"`
}

// Links holds pagination links in DigitalOcean API responses.
type Links struct {
	Pages map[string]string `json:"pages,omitempty"`
}

// Meta holds pagination metadata in DigitalOcean API responses.
type Meta struct {
	Total int `json:"total,omitempty"`
}

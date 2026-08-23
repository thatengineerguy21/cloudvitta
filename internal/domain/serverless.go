package domain

// Canonical CPU architecture constants for serverless compute.
const (
	ArchitectureX86_64 = "x86_64"
	ArchitectureARM64  = "arm64"
)

// Canonical component type constants for serverless pricing observations.
const (
	ComponentTypeRequestFee  = "request_fee"
	ComponentTypeDurationFee = "duration_fee"
)

// Canonical tier constants for serverless compute.
const (
	ServerlessTierConsumption     = "consumption"
	ServerlessTierFlexConsumption = "flex_consumption"
	ServerlessTier1stGen          = "1st_gen"
	ServerlessTier2ndGen          = "2nd_gen"
)

// ServerlessRateAttributes holds normalized attributes for a serverless
// compute RATE observation as ingested from a provider. It describes the
// SKU/meter, not a caller's workload — there is no memory, duration, or
// request-count value on a rate row.
type ServerlessRateAttributes struct {
	Architecture  string `json:"architecture"`   // "x86_64", "arm64"
	Tier          string `json:"tier,omitempty"` // "consumption", "flex_consumption", "1st_gen", "2nd_gen"
	ComponentType string `json:"component_type"` // "request_fee", "duration_fee"
}

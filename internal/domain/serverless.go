package domain

// Canonical CPU architecture constants for serverless compute.
const (
	ArchitectureX86_64 = "x86_64"
	ArchitectureARM64  = "arm64"
)

// Canonical component type constants for serverless pricing observations.
const (
	ComponentTypeRequestFee        = "request_fee"
	ComponentTypeDurationFee       = "duration_fee"        // AWS and Azure undifferentiated duration meter
	ComponentTypeDurationFeeCPU    = "duration_fee_cpu"    // GCP GHz-seconds (1st Gen) or vCPU-seconds (2nd Gen)
	ComponentTypeDurationFeeMemory = "duration_fee_memory" // GCP GB-seconds (1st Gen) or GiB-seconds (2nd Gen)
)

// Canonical unit constants for serverless pricing observations.
const (
	UnitPerMillionRequests = "per_million_requests"
	UnitPerRequest         = "per_request"
	UnitPer10Requests      = "per_10_requests"
	UnitPerGBSecond        = "per_gb_second"
	UnitPerGHzSecond       = "per_ghz_second"
	UnitPerVCPUSecond      = "per_vcpu_second"
	UnitPerGiBSecond       = "per_gib_second"
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
	ComponentType string `json:"component_type"` // "request_fee", "duration_fee", "duration_fee_cpu", "duration_fee_memory"
	Unit          string `json:"unit,omitempty"` // "per_million_requests", "per_request", "per_10_requests", "per_gb_second", etc.
}

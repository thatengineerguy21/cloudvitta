package domain

// KubernetesAttributes holds normalized attributes for managed
// Kubernetes control-plane pricing. Worker-node compute is priced
// separately via the compute category and is intentionally not
// represented here.
type KubernetesAttributes struct {
	Tier            string `json:"tier"`                       // "free", "standard", "extended_support"
	ClusterTopology string `json:"cluster_topology,omitempty"` // "zonal", "regional", "autopilot" — GCP-specific
}

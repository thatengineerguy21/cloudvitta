package domain

// KubernetesTier represents canonical Kubernetes cluster management tier.
type KubernetesTier string

const (
	KubernetesTierFree            KubernetesTier = "free"
	KubernetesTierStandard        KubernetesTier = "standard"
	KubernetesTierExtendedSupport KubernetesTier = "extended_support"
)

// ClusterTopology represents Kubernetes cluster topology / deployment model.
type ClusterTopology string

const (
	ClusterTopologyZonal     ClusterTopology = "zonal"
	ClusterTopologyRegional  ClusterTopology = "regional"
	ClusterTopologyAutopilot ClusterTopology = "autopilot"
)

// KubernetesAttributes holds normalized attributes for managed
// Kubernetes control-plane pricing. Worker-node compute is priced
// separately via the compute category and is intentionally not
// represented here.
type KubernetesAttributes struct {
	Tier            KubernetesTier  `json:"tier"`                       // "free", "standard", "extended_support"
	ClusterTopology ClusterTopology `json:"cluster_topology,omitempty"` // "zonal", "regional", "autopilot" — GCP-specific
}

# 34. Kubernetes Conditional Control-Plane Credit

Date: 2026-08-23

## Status

Accepted

## Context

Cloud providers charge different control-plane management fees for managed Kubernetes services:
1. **AWS EKS**: Charges a fixed $0.10/hour fee per cluster.
2. **Azure AKS**: Offers a free standard control plane ($0.00/hour), charging $0.10/hour only when the optional Uptime SLA tier is enabled.
3. **GCP GKE**: Charges a $0.10/hour management fee per cluster, but provides a **$74.40/month billing account credit** that offsets the entire management fee for one zonal cluster or Autopilot cluster. Regional GKE clusters and additional clusters incur the full $0.10/hour management fee.

Applying the $74.40/month credit unconditionally to all GKE clusters misrepresents multi-cluster or regional cluster costs as zero. Conversely, never applying the credit misrepresents single zonal cluster costs as $0.10/hour.

## Decision

We condition the GKE management fee credit strictly on caller-specified cluster topology:

1. **Topology Condition Rules**:
   - If the caller specifies `cluster_topology = "zonal"` or `cluster_topology = "autopilot"`, the system applies the $74.40/month ($0.10/hour) credit, resulting in a net $0.00/hour control-plane cost.
   - If the caller specifies `cluster_topology = "regional"`, zero credit is applied, resulting in the standard $0.10/hour management fee.
   - If `cluster_topology` is empty or unspecified, zero credit is applied, and the response includes an explicit honesty warning (`code: "cluster_topology_unspecified"`, `message: "GKE management fee credit requires explicit cluster_topology ('zonal' or 'autopilot'). Applied full management fee ($0.10/hr)."`).

2. **Provider Strategy Encapsulation**:
   - Credit evaluation is encapsulated inside the `KubernetesCostAdjuster` strategy pattern (`GKEFeeAdjuster`), keeping the core `KubernetesScorer` free of provider-specific conditional branches.

## Consequences

### Positive
- Strict compliance with ADR 0022 Honesty Contract: no silent assumption of cluster topology or free management credits.
- Provides actionable warnings that educate clients on how to specify topology for optimal cost modeling.
- Clear encapsulation of provider billing idiosyncrasies behind strategy interfaces.

### Negative
- Callers must provide the `cluster_topology` parameter to observe the GKE billing credit in comparison results.

## Alternatives Considered

### Alternative 1: Unconditional Flat Credit Subtraction
Rejected. Automatically zeroing out GKE management fees violates the Honesty Contract because regional GKE clusters incur standard hourly management fees.

### Alternative 2: Disallow Topology Input and Ignore Credit
Rejected. Overstates costs for standard single-cluster zonal workloads which represent the majority of small-to-medium GKE deployments.

# 45. Big 3 Multi-Region Ingestion and Capacity Management

Date: 2026-09-07

## Status

Accepted

## Context

CloudVitta initially normalized cloud provider pricing for primary US regions only. However, cloud infrastructure users deploy workloads globally. Providers charge different prices for the same machine types, storage classes, and network egress across global regions. To provide accurate comparisons, CloudVitta must ingest multi-region pricing from the three major cloud providers (Amazon Web Services, Microsoft Azure, and Google Cloud Platform).

Full multi-region ingestion without constraints creates three operational risks:
1. **Database Table Bloat**: Ingesting all global provider regions (more than 40 regions per provider) would create millions of rows, exceeding the capacity of the serverless PostgreSQL free tier.
2. **Storage Payload Costs**: Unmanaged storage of raw provider responses in Google Cloud Storage would cause storage costs to grow over time.
3. **Cloud Run Memory and Timeout Failures**: Concurrent large payload downloads could exceed the memory limit and the default 15-minute job execution timeout of Google Cloud Run.

We need an architecture that supports multi-region pricing across the three major providers while maintaining strict capacity boundaries.

## Decision

We establish multi-region ingestion across eight strategic global infrastructure hubs with capacity controls and lifecycle management:

1. **Strategic Global Hubs Taxonomy**:
   - We define eight strategic global hubs in [`internal/matching/regionmap/hubs.go`](file:///D:/02-code/cloudvitta/internal/matching/regionmap/hubs.go):
     - US East (Virginia): `us-east-1` (AWS), `eastus` (Azure), `us-east4` (GCP).
     - US West (Oregon): `us-west-2` (AWS), `westus2` (Azure), `us-west1` (GCP).
     - Europe (Frankfurt): `eu-central-1` (AWS), `germanywestcentral` (Azure), `europe-west3` (GCP).
     - UK (London): `eu-west-2` (AWS), `uksouth` (Azure), `europe-west2` (GCP).
     - Asia (Singapore): `ap-southeast-1` (AWS), `southeastasia` (Azure), `asia-southeast1` (GCP).
     - Asia (Tokyo): `ap-northeast-1` (AWS), `japaneast` (Azure), `asia-northeast1` (GCP).
     - India (Mumbai): `ap-south-1` (AWS), `centralindia` (Azure), `asia-south1` (GCP).
     - Australia (Sydney): `ap-southeast-2` (AWS), `australiaeast` (Azure), `australia-southeast1` (GCP).
   - Functions `TargetHubs()`, `TargetAWSRegions()`, `TargetAzureRegions()`, `TargetGCPRegions()`, and `IsTargetRegion()` provide the single source of truth.

2. **Compute Hardware Workload Family Classification**:
   - We expand `DetermineCategory()` in [`internal/service/catalog_sync.go`](file:///D:/02-code/cloudvitta/internal/service/catalog_sync.go) to classify all eight workload families:
     - General Purpose (`general_purpose`)
     - Compute Optimized (`compute_optimized`)
     - Memory Optimized (`memory_optimized`)
     - Storage Optimized (`storage_optimized`)
     - GPU / Accelerated (`gpu_accelerated`)
     - High Performance Computing (`hpc`)
     - Network Optimized (`network_optimized`)
     - Burstable (`burstable`)
   - We detect GPU models, GPU counts, and CPU burstable credits across all three providers.

3. **Storage and Network Group Taxonomy**:
   - We classify storage products into eight canonical storage groups (`object`, `block`, `file`, `archive`, `backup_dr`, `hybrid`, `migration`, `specialized_hpc`) with provisioned throughput in MB/s in [`internal/matching/storageclassmap/groups.go`](file:///D:/02-code/cloudvitta/internal/matching/storageclassmap/groups.go).
   - We classify network products into fourteen canonical service groups in [`internal/matching/transfertypemap/groups.go`](file:///D:/02-code/cloudvitta/internal/matching/transfertypemap/groups.go).

4. **Multi-Region Provider Adapters**:
   - **AWS Adapter**: Iterates sequentially over `TargetAWSRegions()`. The adapter fetches regional price list files dynamically using `BuildRegionalURL()` and saves raw payloads to GCS before normalization.
   - **Azure Adapter**: Iterates sequentially over `TargetAzureRegions()` using `armRegionName eq '{region}'` filters for regional categories, and retains global queries for bandwidth.
   - **GCP Adapter**: Ingests the global catalog stream and filters observations using `regionmap.IsTargetRegion("gcp", region)` to drop non-target regions before database upsert.

5. **Concurrency and Timeout Hardening**:
   - We set `DefaultOrchestratorConfig().MaxConcurrency = 3` in [`internal/service/ingest_orchestrator.go`](file:///D:/02-code/cloudvitta/internal/service/ingest_orchestrator.go) to prevent Cloud Run memory exhaustion.
   - We increase the ingestion timeout in [`cmd/ingest/main.go`](file:///D:/02-code/cloudvitta/cmd/ingest/main.go) and [`.github/workflows/deploy.yml`](file:///D:/02-code/cloudvitta/.github/workflows/deploy.yml) from 15 minutes to 35 minutes.

6. **Automated Storage Lifecycle Management**:
   - We automate object lifecycle policies on Google Cloud Storage via [`scripts/setup-gcs-lifecycle.sh`](file:///D:/02-code/cloudvitta/scripts/setup-gcs-lifecycle.sh).
   - Objects in the `raw/` directory move to `NEARLINE` storage class after 30 days.
   - Objects move to `COLDLINE` storage class after 90 days.
   - Objects expire and delete permanently after 180 days.
   - This caps total raw storage at ~3.4 GB compressed, maintaining storage costs below $0.08 per month.

7. **Cross-Protocol Parity**:
   - Cross-protocol test suite [`internal/transport/mcp/contract_test.go`](file:///D:/02-code/cloudvitta/internal/transport/mcp/contract_test.go) verifies exact output equivalence between REST and MCP across all eight hubs.

## Consequences

### Positive
- **Global Accuracy**: Cost comparisons reflect actual regional cloud prices in North America, Europe, and Asia Pacific.
- **Controlled Growth**: Filtering to eight strategic hubs maintains the total active SKU count at ~51,272 rows and PostgreSQL storage at ~44.7 MB, well within free-tier limits.
- **Predictable Costs**: Automated GCS lifecycle transitions prevent long-term storage cost accumulation.
- **Resource Protection**: Bounded concurrency of 3 workers protects Cloud Run container memory from exhaustion.
- **Protocol Parity**: REST API users and Model Context Protocol (MCP) clients receive identical pricing and warning structures.

### Negative
- Cloud Run ingestion executions take longer (up to 30 minutes) because regional price lists stream sequentially.
- Unmapped regions outside the eight hubs are dropped unless explicitly registered in the target hub taxonomy.

## Alternatives Considered

1. **Ingesting All 40+ Cloud Regions per Provider**: Rejected because database storage would exceed 250 MB and ingestion would exceed Cloud Run task limits.
2. **Parallel Regional Downloads within Adapters**: Rejected because downloading eight regional files at the same time in multiple goroutines would cause memory spikes above 1.5 GiB.
3. **No Storage Lifecycle Policy**: Rejected because storing all raw historical responses would cost more each month as data accumulates.

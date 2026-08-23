# 33. NoSQL Cross-Unit Workload Normalization

Date: 2026-08-23

## Status

Accepted (Resolves Open Question #21)

## Context

Cloud providers publish managed NoSQL database pricing using incompatible operational and throughput units:
1. **AWS DynamoDB**: Rates are measured in Read/Write Capacity Units (RCU/WCU) for provisioned mode, or raw read/write request units for on-demand mode.
2. **Azure Cosmos DB**: Rates are measured in Request Units per second (RU/s).
3. **GCP Firestore**: Rates are measured in raw document read and write operations.

These disparate metrics cannot be directly matched without establishing a common operational workload baseline. Open Question #21 also questioned whether NoSQL databases should share the relational `/api/v1/prices/database` endpoint or use a dedicated endpoint.

## Decision

We establish cross-unit NoSQL workload normalization rules and a dedicated comparison endpoint:

1. **Dedicated Endpoint**:
   - Serve NoSQL comparisons via `GET /api/v1/prices/database-nosql` (and MCP tool `compare_database_nosql`) to prevent parameter pollution on the relational database endpoint.

2. **Standard 1 KB Baseline Payload Assumption**:
   - Normalize throughput to standard operations using an industry-standard baseline assumption of a **1 KB item/document payload**.

3. **Standardized Throughput Conversion Formulas**:
   - **AWS DynamoDB**:
     - 1 RCU provides 1 strongly consistent read per second (up to 4 KB payload $\rightarrow$ 1 read/sec at 1 KB).
     - 1 WCU provides 1 write per second (up to 1 KB payload $\rightarrow$ 1 write/sec at 1 KB).
   - **Azure Cosmos DB**:
     - 1 point read of a 1 KB document consumes 1 RU.
     - 1 write of a 1 KB document consumes 5 RU.
   - **GCP Firestore**:
     - Raw document operations per second are converted to monthly billable operations ($Units \times 2,628,000\text{ seconds/month}$).

4. **Multi-Component Normalization**:
   - Ingestion normalizes and stores separate `throughput` and `storage` observation rows.
   - `NoSQLScorer` joins throughput and storage candidate rows in memory dynamically at query time based on provider, region, data model (`key_value`, `document`, `multi_model`), and pricing mode (`provisioned`, `on_demand`).

## Consequences

### Positive
- Enables transparent, deterministic apples-to-apples NoSQL cost comparisons across AWS, Azure, and GCP.
- Preserves distinct parameter spaces between relational and NoSQL database engines.
- Clarifies operational assumptions in API contract documentation.

### Negative
- Callers with workloads that deviate substantially from the 1 KB baseline payload (e.g. 50 KB documents) must scale their read/write operations input parameters accordingly.

## Alternatives Considered

### Alternative 1: Overload Relational Endpoint (`GET /api/v1/prices/database?model=nosql`)
Rejected. Relational and NoSQL queries share almost no parameter dimensions (vCPU/RAM vs RCU/WCU/RU/s), resulting in confusing mutual exclusivity rules on a single endpoint.

### Alternative 2: Require Raw Provider-Specific Units
Rejected. Requiring callers to supply provider-native units (e.g. RCU for AWS, RU/s for Azure) eliminates the cross-provider comparison utility of CloudVitta.

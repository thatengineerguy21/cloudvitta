# 30. Database Instance and Storage Query-Time Join

Date: 2026-08-23

## Status

Accepted

## Context

Cloud providers publish managed relational database pricing (AWS RDS/Aurora, Azure Database for PostgreSQL/MySQL/SQL Server, GCP Cloud SQL/AlloyDB) as separate compute instance and storage capacity rate meters.

These meters use different billing dimensions and units:
1. Compute instances are billed per instance-hour.
2. Storage capacity is billed per GB-month.
3. Provisioned IOPS are billed per IOPS-month or IOPS-hour.

If the ingestion worker joins every compute instance with every storage option at ingestion time, it causes a Cartesian product explosion in the `price_observations` table. For example, 100 instance types combined with 10 storage classes and 50 regional permutations generates tens of thousands of redundant rows per provider.

## Decision

We normalize and store compute instance observations and storage capacity observations as separate rows in `price_observations`:

1. **Ingestion Normalization**:
   - Ingestion worker stores compute instance observations with `service_category = "database_rdbms"` and `attributes.component_type = "instance"`.
   - Ingestion worker stores storage capacity observations with `service_category = "database_rdbms"` and `attributes.component_type = "storage"`.

2. **Query-Time In-Memory Join**:
   - The database matching engine (`DatabaseRDBMSScorer`) retrieves both instance and storage candidate slices for the target provider and region.
   - The scorer joins instance and storage candidates in memory based on provider, region, canonical database engine (`postgresql`, `mysql`, `sqlserver`), and storage family.

3. **Combined Hourly Pricing Arithmetic**:
   - The pricing calculation combines the instance hourly rate with the normalized storage hourly rate ($Price_{monthly} / 730$) and provisioned IOPS rate:
     $$\text{Hourly Total} = \text{Price}_{instance} + \left(\frac{\text{StorageGB} \times \text{Price}_{storage\_monthly}}{730}\right) + \left(\frac{\text{IOPS} \times \text{Price}_{iops\_monthly}}{730}\right)$$

## Consequences

### Positive
- Prevents database table explosion and minimizes ingestion payload processing overhead.
- Allows clients to request arbitrary storage capacities and provisioned IOPS dynamically.
- Maintains streaming JSON token processing without memory buffering.

### Negative
- `DatabaseRDBMSScorer` must perform an in-memory candidate join on cache miss or cache read.

## Alternatives Considered

### Alternative 1: Ingestion-Time Cartesian Permutation Generation
Rejected. Generating all permutations at ingestion time increases database storage requirements by more than 50x and restricts storage sizes to arbitrary pre-computed steps.

### Alternative 2: Rigid Fixed-Storage Baseline Rows
Rejected. Fixing database rows to a static 100 GB storage capacity prevents accurate workload modeling for callers with distinct storage or high-IOPS needs.

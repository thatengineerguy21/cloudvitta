# 8. Serverless Postgres (Neon) for Normalized Storage

Date: 2026-08-06

## Status

Accepted

## Context

CloudVitta requires a normalized relational database to store ingested price observations. In an enterprise environment, a managed database such as Google Cloud SQL or AWS RDS is provisioned to ensure high availability and consistent baseline latency.

Cost sustainability is a primary constraint. A continuously running managed database instance incurs a fixed monthly baseline cost regardless of traffic volume. Serverless Postgres (Neon) scales compute resources down to zero when idle.

## Decision

We use **Neon (Serverless Postgres)** for the normalized relational database layer.

## Consequences

### Positive
- Substantially reduces baseline infrastructure costs by billing only for active compute time and stored bytes.
- Eliminates manual database scaling and virtual machine maintenance.
- Aligns with serverless economics across the entire application stack.

### Negative
- **Cold Starts**: When the database scales down to zero, API requests that encounter a cache miss during idle periods experience a latency spike (several seconds) while the database resumes compute.
- We explicitly accept this cold-start delay to maintain zero idle costs.

## Alternatives Considered

### Alternative 1: Always-On Managed PostgreSQL (Google Cloud SQL / AWS RDS)
Rejected. Managed cloud database instances require continuous baseline billing (approximately $30-$60/month minimum), conflicting with scale-to-zero budget requirements for this system.

### Alternative 2: NoSQL / Document Store (MongoDB / AWS DynamoDB)
Rejected. Cloud pricing comparisons require multi-dimensional filtering, range queries, and schema constraints. SQL relational storage with `sqlc` generates type-safe queries and compile-time verification that NoSQL systems lack.

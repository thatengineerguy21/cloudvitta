# 8. Serverless Postgres (Neon) for Normalized Storage

Date: 2026-08-06

## Status

Accepted

## Context

CloudVitta requires a normalized relational database to store ingested price observations. In a traditional enterprise environment, a managed database like Google Cloud SQL or AWS RDS would be provisioned to ensure high availability and consistent low latency.

However, as a portfolio project, cost optimization is a primary constraint. A permanently running Cloud SQL instance carries a fixed baseline monthly cost, regardless of traffic. Serverless Postgres (Neon) offers an alternative that scales compute down to zero when idle.

## Decision

We will use **Neon (Serverless Postgres)** for our normalized database layer.

## Consequences

### Positive
- Massive reduction in baseline infrastructure costs; we only pay for active compute and storage.
- Eliminates the need to manage infrastructure provisioning or manual database scaling.
- Provides a great talking point in interviews regarding cost-aware architecture.

### Negative
- **Cold Starts**: Because the database scales to zero, API requests that trigger a Redis cache-miss while the database is asleep will incur a significant latency spike (several seconds) while Postgres spins back up. 
- We explicitly accept this cold start penalty as a valid trade-off for a portfolio project, prioritizing budget sustainability over absolute p99 latency guarantees.

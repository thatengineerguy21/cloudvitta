# 17. Native Concurrency with Idempotent Checkpoints

Date: 2026-08-06

## Status

Accepted

## Context

CloudVitta's ingestion process fetches pricing data for millions of SKUs across 7 cloud providers. We needed to decide how to manage this concurrency and handle crash recovery (e.g., an OOM kill or Cloud Run timeout midway through a 30-minute fetch cycle).

Options included adopting a distributed, general-purpose job queue framework (e.g., RabbitMQ, Celery, or machinery) to manage workers, retries, and task state, versus relying strictly on native Go concurrency tools (`errgroup`, `rate.Limiter`, `singleflight`) combined with a custom Dead Letter Queue.

While native Go concurrency is highly performant locally, `errgroup` loses its in-memory state if the process crashes, meaning a restarted container would theoretically have to begin the ingestion cycle from scratch.

## Decision

We will strictly use **Native Go Concurrency** combined with an **External Idempotent Checkpoint Table**, explicitly rejecting general-purpose job queue frameworks.

1. **Native Concurrency**: We use `errgroup` for parallel fetches and `rate.Limiter` to respect upstream API limits.
2. **Idempotent Chunking**: The ingestion workload is decomposed into small, idempotent units (e.g., per provider, category, and region).
3. **Checkpoint Table**: Completed chunks are recorded externally (e.g., a lightweight progress table in Postgres or Redis keyed by the run ID).
4. **Crash Recovery**: On process restart (handled natively by Cloud Run Jobs), the worker skips completed chunks and resumes fetching the rest.

## Consequences

### Positive
- **Avoids Heavy Infrastructure**: A distributed job queue requires an always-on message broker and worker fleet to operate, directly contradicting our scale-to-zero serverless constraints (Neon, Grafana Cloud).
- **Correct Scaling Bottleneck**: The actual bottleneck in ingestion is the *upstream provider API rate limits*, not local CPU capacity. Scaling out distributed workers via a job queue wouldn't buy more throughput; they would just hit the upstream API ceiling collectively.
- **Safe Retries**: Because our storage model uses upsert-with-history (ADR-006), if checkpointing is imprecise, re-fetching an already-done chunk simply redundantly upserts current data without corrupting historical fidelity.

### Negative
- Requires building a small amount of custom checkpoint/resume logic rather than getting it out-of-the-box from a framework like RabbitMQ.

## Alternatives Considered

### Alternative 1: Distributed Job Queue and Message Brokers (RabbitMQ / Celery / Machinery)
Rejected. Distributed message brokers require continuously running broker and worker instances, conflicting with scale-to-zero serverless constraints. Ingestion bottlenecks stem from upstream provider rate limits rather than local worker concurrency.

### Alternative 2: Single-Threaded Sequential Ingestion
Rejected. Sequential ingestion across all cloud providers, categories, and regions takes hours to complete, exceeding Cloud Run execution timeout ceilings and delaying pricing data availability.

# 18. Lightweight Redis DLQ

Date: 2026-08-06

## Status

Accepted

## Context

During the ingestion process, if a provider's data fails to parse correctly, the failure must be recorded for triage. The architecture mandates a custom, purpose-built Redis-backed Dead Letter Queue (`ingestion:dlq`) holding the provider, category, timestamp, last error, and consecutive-failure count.

We needed to decide whether to also include the raw JSON payload that caused the failure directly in the Redis DLQ, or explicitly exclude it to keep the DLQ lightweight. Including the payload makes immediate debugging easier, but raw provider JSON files can be hundreds of megabytes in size.

## Decision

We will use a **Payload-Free DLQ**. The Redis DLQ will strictly exclude the raw JSON artifacts. Instead, it will store a `gcs_uri` or lookup key alongside the error string.

## Consequences

### Positive
- **Memory Optimization**: Redis is an in-memory datastore with strict quotas (Upstash). Storing hundreds of megabytes of raw JSON per failed job would rapidly trigger OOM evictions or hit quota limits, taking down the active caching layer. 
- **Leverages Existing Architecture**: Since ADR-0012 mandates storing raw responses in GCS anyway, the DLQ simply points to the GCS blob. This provides full debugging reproducibility with just one extra hop, without duplicating data.
- **Fast Triage**: The DLQ remains a fast, easily queryable list of string metadata for alerting.

### Negative
- Requires a two-step debugging process for on-call developers: query the DLQ in Redis to get the failure context, then fetch the raw payload from GCS using the provided URI. This minor friction is explicitly accepted to protect Redis memory.

## Alternatives Considered

### Alternative 1: Store Full Raw JSON Payloads in Redis DLQ Entries
Rejected. Raw upstream JSON responses reach hundreds of megabytes in size. Storing raw payloads directly in Redis rapidly exhausts memory limits, causes cache key evictions, and burns monthly Upstash command and storage quotas.

### Alternative 2: Discard Failure Details and Rely Only on Terminal Logs
Rejected. Terminal logs lack an actionable, structured query queue for automated status monitoring. The DLQ enables the `GET /api/v1/providers/{provider}/status` endpoint and automated monitoring agents to detect degraded or blocked ingestion state immediately.

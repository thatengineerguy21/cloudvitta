# 12. Raw Response Storage in GCS

Date: 2026-08-06

## Status

Accepted

## Context

CloudVitta ingests pricing data from upstream cloud provider APIs. Once the JSON payload is parsed, normalized, and inserted into Postgres, the raw JSON payload could theoretically be discarded. 

However, parsing bugs are inevitable (e.g., missing a crucial attribute or misinterpreting a new provider-specific pricing tier). We must decide whether to store the raw, unparsed JSON responses persistently, or to simply rely on the next scheduled cron job to refetch and correct the data.

## Decision

We will store all raw JSON responses from upstream providers in **Google Cloud Storage (GCS)**, organized by date, provider, and category, with a 90-day pruning lifecycle policy.

## Consequences

### Positive
- **Historical Recovery for Volatile Prices**: "Refetch tomorrow" only works for static On-Demand prices. Spot pricing is highly volatile; a parsing bug today cannot be fixed by tomorrow's fetch, because tomorrow's fetch returns a different price. Storing the raw payload guarantees we can replay and correct historical spot price data.
- **Auditability**: Acts as an undeniable audit trail bridging what the provider returned versus what CloudVitta parsed.
- **Low Cost**: GCS storage for text files with an aggressive 90-day expiration lifecycle costs fractions of a cent, making the insurance virtually free.

### Negative
- Requires a secondary storage bucket and explicit lifecycle configuration.
- Requires building a small "re-ingest from GCS" script/tool for developers to use when a parsing bug is actually discovered.

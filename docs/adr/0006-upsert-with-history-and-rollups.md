# 6. Upsert-with-History and Rollups Over Append-Only Postgres

Date: 2026-08-06

## Status

Accepted

## Context

CloudVitta ingests pricing data for millions of SKUs across 7 cloud providers. The initial data model proposed an "Append-only" architecture for the `price_observations` table in Postgres (Neon), where every successful ingestion run would insert a brand new row even if the price hadn't changed. 

The original justification was a "perfect historical audit trail." However, upon review, this use case doesn't practically exist: customers consume aggregates (trend charts, min/max/avg), not raw ticks. Furthermore, On-Demand and Reserved prices rarely change, meaning the vast majority of append-only inserts would be entirely redundant. Spot pricing is the only volatile category, and we do not need hourly-plus granularity for Spot instances extending beyond two weeks.

## Decision

We will use an **Upsert-with-History and Rollups** pattern for the `price_observations` table instead of an Append-only pattern.

1. **Upsert-with-History**: We will only insert a *new* row into Postgres when an actual price change is detected. 
2. **Bump Timestamps**: On unchanged fetches, we will simply perform a cheap update to bump the `last_seen_at` timestamp on the existing active row, avoiding index bloat.
3. **Capped Raw Retention & Rollups**: We will cap the retention of raw `price_observations` rows at ~2 weeks. Older data will be rolled up into daily OHLC (Open-High-Low-Close) aggregates, and the raw rows will be dropped.

## Consequences

### Positive
- Massive reduction in redundant data, directly lowering serverless Postgres (Neon) storage costs.
- Postgres indexes stay lean and cache-miss queries remain highly performant.
- Provides a sustainable, steady-state database size rather than unbounded exponential growth.
- Perfectly supports the actual customer-facing feature (aggregate trend analysis) without over-engineering raw data retention.

### Negative
- Requires a slightly more complex ingestion query (`INSERT ... ON CONFLICT DO UPDATE` or checking existing state) compared to a blind `INSERT`.
- Requires a background cron task or database lifecycle rule to perform the daily OHLC rollups and purge raw data older than 2 weeks.

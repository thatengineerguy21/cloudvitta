# 35. Foreign Exchange Rate Ingestion and Persistence Architecture

Date: 2026-08-24

## Status

Accepted

## Context

Cloud provider pricing APIs publish catalog prices in different currencies. For example, AWS, Azure, and GCP publish default prices in USD, while Alibaba Cloud returns native prices in CNY for Mainland China regions. Additionally, international API consumers require price comparisons in their local currencies.

CloudVitta requires an unmetered, highly available Foreign Exchange (FX) service to convert prices accurately without introducing floating-point precision loss or runtime vendor lock-in.

## Decision

We implement a dedicated `FXService` (`internal/fx`) with multi-tier caching and PostgreSQL persistence:

1. **Primary Upstream Provider**:
   - We query the public Frankfurter API (`https://api.frankfurter.dev/v1/latest?base=USD`), which publishes official European Central Bank (ECB) reference exchange rates.
   - The API is unmetered, requires zero authentication secrets, and publishes daily rate updates on ECB business days.

2. **Persistence Schema & Synchronization**:
   - We create the `fx_rates` table via forward-only database migration (`migrations/0003_create_fx_rates.sql`).
   - The ingestion runner (`cmd/ingest`) and API server (`cmd/api`) synchronize daily rates into `fx_rates` using SQLC-generated upsert queries (`UpsertFXRate`).

3. **In-Memory Cache & Transparent Database Fallback**:
   - `FXService` maintains an in-memory rate map protected by `sync.RWMutex`.
   - If the upstream Frankfurter API is unavailable, `FXService` falls back to the latest exchange rates recorded in PostgreSQL and marks the returned metadata with `IsFallback: true`.
   - If fallback rates are used during price comparison, the response includes an explicit honesty warning (`code: "stale_fx_rate"`).

4. **Exact Decimal Arithmetic**:
   - All conversions use `github.com/shopspring/decimal` with exact multiplication-before-division order (`amount.Mul(toRate).Div(fromRate)`) to prevent premature rate truncation.

## Consequences

### Positive
- Zero secret management friction for upstream foreign exchange rate queries.
- High resilience: offline development environments and temporary upstream outages transparently use cached PostgreSQL rates.
- Exact monetary precision across direct, inverse, and cross-currency conversions.
- Full compliance with the ADR 0022 Honesty Contract.

### Negative
- Daily rate updates depend on ECB business day publication schedules (~16:00 CET).

## Alternatives Considered

### Alternative 1: Commercial FX APIs with Mandatory API Keys
Rejected. APIs such as Fixer.io or Open Exchange Rates introduce rate limits and secret management complexity on Cloud Run without providing higher rate accuracy than official ECB data.

### Alternative 2: Live Upstream HTTP Query on Every Calculation
Rejected. Increases API latency and introduces a single point of failure on the critical comparison request path.

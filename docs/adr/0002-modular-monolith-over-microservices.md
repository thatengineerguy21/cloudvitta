# 2. Modular Monolith over Microservices

Date: 2026-08-06

## Status

Accepted

## Context

CloudVitta requires two primary runtime execution contexts:
1. A public-facing API serving HTTP/gRPC/MCP traffic (`cmd/api`).
2. A background ingestion worker triggered on a schedule to fetch upstream provider prices (`cmd/ingest`).

While these two components have different scaling characteristics and triggers, they operate over the exact same business domain, database schema, and caching logic. Choosing a microservices architecture would force us to introduce network boundaries between the ingest logic and the store logic, or duplicate struct definitions and database drivers across repositories.

## Decision

We will use a **Modular Monolith** architecture.
- Both `cmd/api` and `cmd/ingest` live in the same Go repository.
- They share the `internal/` packages (e.g., `internal/store`, `internal/domain`, `internal/cache`).
- They are compiled into separate binaries and deployed as separate containers (Cloud Run for API, Cloud Scheduler -> Cloud Run Job for Ingest), allowing independent scaling without independent codebases.

### Database Migration Strategy (Expand-Contract)
Because `cmd/api` and `cmd/ingest` deploy independently but share a database, a schema change deployed to one before the other could cause downtime or panic. To mitigate this, we decided:
1. **Decoupled Migrations**: Migrations run as a separate, deliberate step in CI/CD *before* rolling out new container versions.
2. **Expand-Contract Pattern**: All migrations must be strictly additive. We will add nullable columns or new tables first (Expand). We will only drop old columns/tables (Contract) in a subsequent deployment after all containers have migrated to the new schema.
3. **Backward-Compatible Structs**: Go structs must handle missing or extra fields gracefully during the rollout window where old and new container versions coexist against the intermediate database state.

## Consequences

### Positive
- Single source of truth for all domain models and database queries.
- No network overhead or complex distributed tracing needed to debug data flow from ingestion to serving.
- Simplified local development (one `go build`, one `.env` file).

### Negative
- Care must be taken not to let `cmd/api` import `cmd/ingest` specific packages or vice-versa.
- Strict discipline required for database migrations (Expand-Contract) to prevent deployment race conditions, as both binaries share the schema.

# 2. Modular Monolith over Microservices

Date: 2026-08-06

## Status

Accepted

## Context

CloudVitta requires two primary runtime execution contexts:
1. A public-facing API serving REST and MCP traffic (`cmd/api`).
2. A background ingestion worker triggered on a schedule to fetch upstream provider prices (`cmd/ingest`).

While these two components have different scaling characteristics and triggers, they operate over the same business domain, database schema, and caching logic. A microservices architecture introduces network boundaries between the ingest logic and the store logic, and duplicates struct definitions and database drivers across repositories.

## Decision

We use a **Modular Monolith** architecture.
- Both `cmd/api` and `cmd/ingest` reside in the same Go repository.
- Both binaries share the `internal/` packages (`internal/store`, `internal/domain`, `internal/cache`, `internal/service`).
- Both binaries compile into separate binaries and deploy as separate containers (Cloud Run for API, Cloud Scheduler to Cloud Run Job for Ingest). This enables independent scaling without independent codebases.

### Database Migration Strategy (Expand-Contract)
Because `cmd/api` and `cmd/ingest` deploy independently but share a database, a schema change deployed to one container before the other can cause downtime or panics. To mitigate this:
1. **Decoupled Migrations**: Migrations run as a separate, deliberate step in CI/CD before rolling out new container versions.
2. **Expand-Contract Pattern**: All migrations are additive. We add nullable columns or new tables first (Expand). We only drop old columns or tables (Contract) in a subsequent deployment after all containers run the new schema.
3. **Backward-Compatible Structs**: Go structs handle missing or extra fields gracefully during the rollout window when old and new container versions coexist against the intermediate database state.

## Consequences

### Positive
- Single source of truth for all domain models and database queries.
- Eliminates network overhead and complex distributed tracing between ingestion and serving.
- Simplifies local development with a single Go workspace and shared environment configuration.

### Negative
- Maintainers must ensure `cmd/api` does not import `cmd/ingest` specific packages or vice-versa.
- Strict discipline is required for database migrations (Expand-Contract) to prevent deployment race conditions across shared schemas.

## Alternatives Considered

### Alternative 1: Separate Microservices per Provider / Protocol
Rejected. Separate microservices duplicate domain models and pricing calculation logic, add network latency between services, increase deployment overhead across multiple repositories, and complicate distributed tracing during initial development.

### Alternative 2: Serverless Functions (AWS Lambda / Google Cloud Functions per Route)
Rejected. Serverless functions introduce cold-start latency spikes, exhaust database connection pools against serverless PostgreSQL (Neon), and fragment cache warming orchestration across isolated execution contexts.

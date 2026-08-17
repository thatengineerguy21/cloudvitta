# 28. Forward-Only Database Migrations

Date: 2026-08-13

## Status

Accepted

## Context

Database schema migrations often write both "up" and "down" scripts. In continuous deployment environments and serverless databases (Neon PostgreSQL), executing automated rollback ("down") scripts in production introduces data loss risks, table locking issues, and complex schema drift when rollback logic is incorrect.

## Decision

We use **Forward-Only Migrations** for all database schema changes in CloudVitta:

1. **No Rollback Scripts**: Migration files in `migrations/` do not contain "down" rollback SQL sections.
2. **Forward Fixes**: If a schema change creates a problem or needs reversion, developers create a new "up" migration script applied sequentially on top of the existing schema.
3. **Additive Operations**: Database schema changes favor non-breaking additive updates before removing deprecated tables or columns (Expand-Contract pattern).

## Consequences

### Positive
- Eliminates the risk of catastrophic data loss from automated rollback scripts executing against live databases.
- Simplifies schema migration tracking and reproducible `sqlc` schema compilation.
- Ensures migration history in production matches source control migration files exactly.

### Negative
- Schema rollbacks require authoring, testing, and deploying a new forward migration script.

## Alternatives Considered

### Alternative 1: Automated Down Migration Rollback Scripts
Rejected. Automated down scripts frequently fail to account for data inserted between deployment and rollback, creating table locks, destructive column drops, and unrecoverable schema corruption.

### Alternative 2: Manual Ad-Hoc SQL Modifications in Production
Rejected. Applying manual SQL adjustments directly to production databases bypasses version control, breaks schema reproduction in local development, and corrupts migration state tables.

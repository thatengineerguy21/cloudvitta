# 28. Forward-Only Database Migrations

Date: 2026-08-13

## Status

Accepted

## Context

Database schema migrations often write both "up" and "down" scripts. In continuous deployment environments and serverless databases (Neon PostgreSQL), executing automated rollback ("down") scripts in production introduces data loss risks, table locking issues, and complex schema drift when rollback logic is incorrect.

## Decision

We will use **Forward-Only Migrations** for all database schema changes in CloudVitta:

1. **No Rollback Scripts**: Migration files in `migrations/` do not contain "down" rollback SQL sections.
2. **Forward Fixes**: If a schema change creates a problem or needs reversion, developers create a new "up" migration script applied sequentially on top of the existing schema.
3. **Additive Operations**: Database schema changes favor non-breaking additive updates before removing deprecated tables or columns.

## Consequences

- Reduces risk of data loss from automated database rollbacks.
- Simplifies schema migration tracking and `sqlc` schema compilation.
- Requires team members and AI agents to write corrections as new forward migration files.

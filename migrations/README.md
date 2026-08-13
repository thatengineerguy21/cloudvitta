# Database Migrations

This directory contains SQL schema migration scripts for PostgreSQL (`sqlc` schema source).

## Forward-Only Migration Policy

CloudVitta follows a **Forward-Only Migration** strategy:

1. **No Down Migrations**: Migration files do not include `down` rollback sections or separate rollback SQL files.
2. **Additive Schema Fixes**: If a schema change causes an issue or requires reversion, create a new forward migration script (e.g., `0002_fix_schema.sql`) applied sequentially on top of the current schema.
3. **Immutability**: Applied migration scripts are append-only. Do not edit previously applied migration files in production environments.

-- 0004_create_compute_instance_catalog.sql
-- Strategy: Forward-Only Migration (No rollback/down migration script, ADR 0028).
-- Purpose: Creates the compute_instance_catalog table and performance indexes.

CREATE TABLE IF NOT EXISTS compute_instance_catalog (
    id                  BIGSERIAL PRIMARY KEY,
    provider            TEXT NOT NULL,
    instance_type_id    TEXT NOT NULL,
    display_name        TEXT NOT NULL,
    instance_family     TEXT NOT NULL,
    category            TEXT NOT NULL,
    vcpu                NUMERIC(6, 2) NOT NULL,
    memory_gib          NUMERIC(8, 2) NOT NULL,
    cpu_architecture    TEXT NOT NULL DEFAULT 'x86_64',
    gpu_count           INTEGER NOT NULL DEFAULT 0,
    gpu_type            TEXT,
    is_burstable        BOOLEAN NOT NULL DEFAULT FALSE,
    is_current_gen      BOOLEAN NOT NULL DEFAULT TRUE,
    first_seen_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_seen_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    attributes          JSONB,
    UNIQUE (provider, instance_type_id)
);

-- Index for provider inventory counts and category filtering
CREATE INDEX IF NOT EXISTS idx_compute_catalog_provider_cat
    ON compute_instance_catalog (provider, category);

-- Index for hardware spec range queries (vCPU and RAM)
CREATE INDEX IF NOT EXISTS idx_compute_catalog_specs
    ON compute_instance_catalog (vcpu, memory_gib);

-- Index for family grouping
CREATE INDEX IF NOT EXISTS idx_compute_catalog_family
    ON compute_instance_catalog (provider, instance_family);

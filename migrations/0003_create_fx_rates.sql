-- 0003_create_fx_rates.sql
-- Strategy: Forward-Only Migration (No rollback/down migration script, ADR 0028).
-- Purpose: Creates the fx_rates table for tracking daily currency conversion rates.

CREATE TABLE IF NOT EXISTS fx_rates (
    id              BIGSERIAL PRIMARY KEY,
    base_currency   TEXT NOT NULL,
    target_currency TEXT NOT NULL,
    rate            NUMERIC NOT NULL,
    source          TEXT NOT NULL,
    rate_date       DATE NOT NULL,
    fetched_at      TIMESTAMPTZ NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (base_currency, target_currency, rate_date)
);

CREATE INDEX IF NOT EXISTS idx_fx_rates_lookup
    ON fx_rates (base_currency, target_currency, rate_date DESC);

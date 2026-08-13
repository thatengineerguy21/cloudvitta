-- 0001_create_price_observations.sql
-- Creates the price_observations table and its primary lookup index.

CREATE TABLE IF NOT EXISTS price_observations (
    id               BIGSERIAL PRIMARY KEY,
    provider         TEXT NOT NULL,
    service_category TEXT NOT NULL,
    sku_id           TEXT NOT NULL,
    display_name     TEXT NOT NULL,
    region           TEXT NOT NULL,
    region_group     TEXT NOT NULL,
    unit             TEXT NOT NULL,
    price_amount     NUMERIC NOT NULL,
    price_currency   TEXT NOT NULL,
    pricing_model    TEXT NOT NULL,
    attributes       JSONB,
    raw_response_ref TEXT,
    fetched_at       TIMESTAMPTZ NOT NULL,
    last_seen_at     TIMESTAMPTZ NOT NULL,
    anomaly_status   TEXT,
    UNIQUE (provider, sku_id, region, fetched_at)
);

CREATE INDEX IF NOT EXISTS idx_price_lookup
    ON price_observations (provider, service_category, region_group, fetched_at DESC);


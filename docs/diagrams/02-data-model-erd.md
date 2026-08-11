# Data Model (Entity Relationship Diagram)

```mermaid
erDiagram
    price_observations {
        BIGSERIAL id PK
        TEXT provider
        TEXT service_category
        TEXT sku_id
        TEXT display_name
        TEXT region
        TEXT region_group
        TEXT unit
        NUMERIC price_amount
        TEXT price_currency
        TEXT pricing_model
        JSONB attributes
        TEXT raw_response_ref
        TIMESTAMPTZ fetched_at
        TIMESTAMPTZ last_seen_at
        TEXT anomaly_status
    }

    users {
        UUID id PK
        TEXT email
        TEXT password_hash
        TIMESTAMPTZ created_at
    }

    refresh_tokens {
        UUID id PK
        UUID user_id FK
        UUID family_id
        TEXT token_hash
        TIMESTAMPTZ created_at
        TIMESTAMPTZ expires_at
        TIMESTAMPTZ revoked_at
        UUID replaced_by FK
    }

    fx_rates {
        BIGSERIAL id PK
        TEXT base_currency
        TEXT target_currency
        NUMERIC exchange_rate
        TIMESTAMPTZ fetched_at
    }

    users ||--o{ refresh_tokens : "owns"
    refresh_tokens ||--o| refresh_tokens : "replaced_by chain"
```

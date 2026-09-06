# Data Model (Entity Relationship Diagram)

This document details the PostgreSQL relational schema and polymorphic JSONB attribute representations across all seven service categories.

---

## 1. Entity Relationship Diagram

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
        BOOLEAN email_verified
        TIMESTAMPTZ created_at
    }

    verification_tokens {
        UUID id PK
        UUID user_id FK
        TEXT token_hash
        TEXT purpose
        TIMESTAMPTZ expires_at
        TIMESTAMPTZ consumed_at
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
        NUMERIC rate
        TEXT source
        DATE rate_date
        TIMESTAMPTZ fetched_at
        TIMESTAMPTZ created_at
    }

    compute_instance_catalog {
        BIGSERIAL id PK
        TEXT provider
        TEXT instance_type_id
        TEXT display_name
        TEXT instance_family
        TEXT category
        NUMERIC vcpu
        NUMERIC memory_gib
        TEXT cpu_architecture
        INTEGER gpu_count
        TEXT gpu_type
        BOOLEAN is_burstable
        BOOLEAN is_current_gen
        TIMESTAMPTZ first_seen_at
        TIMESTAMPTZ last_seen_at
        JSONB attributes
    }

    users ||--o{ refresh_tokens : "owns"
    users ||--o{ verification_tokens : "owns"
    refresh_tokens ||--o| refresh_tokens : "replaced_by chain"
```

---

## 2. Polymorphic Attribute Codec Architecture

The `attributes` JSONB column stores category-specific specification vectors. Serialization and deserialization are handled through the extensible `RegisterCategoryAttributeCodec` registry (`internal/domain/price_observation.go` per ADR 0031).

```mermaid
flowchart LR
    PO["PriceObservation.Attributes (JSONB)"]
    Reg["Category Attribute Codec Registry"]
    
    C1["ComputeAttributes (compute)"]
    C2["StorageAttributes (storage)"]
    C3["NetworkAttributes (network)"]
    C4["DatabaseRDBMSAttributes (database_rdbms)"]
    C5["DatabaseNoSQLAttributes (database_nosql)"]
    C6["KubernetesAttributes (kubernetes)"]
    C7["ServerlessRateAttributes (serverless)"]
    
    PO --> Reg
    Reg --> C1
    Reg --> C2
    Reg --> C3
    Reg --> C4
    Reg --> C5
    Reg --> C6
    Reg --> C7
```

---

## 3. Polymorphic JSONB Schema Definitions

### 1. Compute (`ComputeAttributes`)
```json
{
  "vcpu": 4,
  "ram_gb": 16.0,
  "family": "general_purpose",
  "gpu_count": 0,
  "storage_type": "ebs_only"
}
```

### 2. Storage (`StorageAttributes`)
```json
{
  "storage_class": "standard",
  "min_duration_days": 0,
  "redundancy": "regional"
}
```

### 3. Network (`NetworkAttributes`)
```json
{
  "transfer_type": "internet_egress",
  "tier_min_gb": 0,
  "tier_max_gb": 10000
}
```

### 4. Managed Relational Databases (`DatabaseRDBMSAttributes` - ADR 0030)
- **Instance Component Row (`component_type = "instance"`):**
  ```json
  {
    "component_type": "instance",
    "engine": "postgresql",
    "vcpu": 4,
    "ram_gb": 16.0,
    "multi_az": false,
    "deployment_tier": "general_purpose"
  }
  ```
- **Storage Capacity Row (`component_type = "storage"`):**
  ```json
  {
    "component_type": "storage",
    "engine": "postgresql",
    "storage_family": "gp3",
    "max_iops": 12000,
    "multi_az": false
  }
  ```

### 5. Managed NoSQL Databases (`DatabaseNoSQLAttributes` - ADR 0033)
- **Throughput Component Row (`component_type = "throughput"`):**
  ```json
  {
    "component_type": "throughput",
    "data_model": "document",
    "pricing_mode": "provisioned",
    "operation_type": "read",
    "multi_region": false
  }
  ```
- **Storage Capacity Row (`component_type = "storage"`):**
  ```json
  {
    "component_type": "storage",
    "data_model": "document",
    "pricing_mode": "provisioned",
    "multi_region": false
  }
  ```

### 6. Managed Kubernetes Control-Plane (`KubernetesAttributes` - ADR 0034)
```json
{
  "tier": "standard",
  "cluster_topology": "zonal"
}
```

### 7. Serverless Compute (`ServerlessRateAttributes`)
- **Request Rate Row (`rate_component = "request_fee"`):**
  ```json
  {
    "rate_component": "request_fee",
    "tier": "standard",
    "architecture": "x86_64",
    "billing_unit": "requests",
    "free_tier_allowance": 1000000
  }
  ```
- **Duration Rate Row (`rate_component = "duration_fee"` or split `duration_fee_cpu` / `duration_fee_memory`):**
  ```json
  {
    "rate_component": "duration_fee",
    "tier": "standard",
    "architecture": "x86_64",
    "billing_unit": "gb_seconds",
    "free_tier_allowance": 400000
  }
  ```

---

## 4. Compute Hardware Instance Catalog (`compute_instance_catalog` - ADR 0041)

The `compute_instance_catalog` table stores slowly changing virtual machine hardware specifications:
- **`provider`**: Canonical provider identifier (`aws`, `azure`, `gcp`, `oracle`, `ibm`, `alibaba`, `digitalocean`).
- **`instance_type_id`**: Provider instance type identifier (for example, `m6i.large`, `Standard_D4s_v5`).
- **`display_name`**: Human-readable name.
- **`instance_family`**: Normalized instance family identifier (for example, `m6i`, `d4s_v5`).
- **`category`**: Normalized hardware category (`general_purpose`, `compute_optimized`, `memory_optimized`, `gpu_accelerated`, `storage_optimized`).
- **`vcpu`**: Number of virtual processor cores.
- **`memory_gib`**: Total memory capacity in gibibytes (GiB).
- **`cpu_architecture`**: Instruction set architecture (`arm64`, `x86_64`).
- **`gpu_count`**: Number of attached graphics processing units.
- **`gpu_type`**: Optional GPU model name.
- **`is_burstable`**: True if CPU performance is credit-based.
- **`is_current_gen`**: True if the provider marks the instance as current generation.
- **`first_seen_at`**: Timestamp when the system first observed this instance type.
- **`last_seen_at`**: Timestamp of the latest price observation ingestion.
- **`attributes`**: Optional JSONB payload with extra hardware dimensions.

---

## 5. Verification Tokens (`verification_tokens` - ADR 0043)

The `verification_tokens` table stores single-use cryptographic tokens for account email verification:
- **`id`**: Unique token identifier (UUID primary key).
- **`user_id`**: Foreign key reference to `users.id` with cascade deletion.
- **`token_hash`**: Hex-encoded SHA-256 hash of the 32-byte cryptographically secure random raw token.
- **`purpose`**: Token purpose identifier (for example, `email_verification`).
- **`expires_at`**: Expiration timestamp (30 minutes after creation).
- **`consumed_at`**: Timestamp when the token was successfully verified (NULL until consumed).
- **`created_at`**: Token creation timestamp.

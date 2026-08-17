# 25. Composed Fail-Fast Configuration

Date: 2026-08-06

## Status

Accepted

## Context

CloudVitta requires robust configuration validation to prevent deploying containers that crash at runtime due to missing environment variables. The coding standards mandate a `Validate()` function that fails fast and crashes the process on startup if the configuration is invalid.

However, the codebase produces multiple binaries (`cmd/api` and `cmd/ingest`) that have overlapping but distinct configuration needs. The API needs JWT secrets but no upstream API keys; the ingestion job needs upstream API keys but no JWT secrets. Using a single monolithic config for the entire repository would force us to provision unnecessary secrets to containers that don't need them, violating the principle of least privilege.

## Decision

We will use **Composed, Binary-Specific Configurations** combined with strict fail-fast validation. 

1. **Sub-Configs**: Shared configurations (e.g., `DBConfig`, `RedisConfig`) are defined as independent structs with their own validation rules.
2. **Binary Composition**: `cmd/api` and `cmd/ingest` each define their own top-level `Config` struct, built by embedding the shared sub-configs alongside binary-specific sub-configs (e.g., `JWTConfig` for the API).
3. **Targeted Validation**: Each binary's `Validate()` method delegates to the `Validate()` methods of its embedded structs. It only checks the config surface it actually uses.

## Consequences

### Positive
- **Least Privilege Maintained**: The ingestion container never needs a JWT secret provisioned to it because the field simply does not exist in its config surface.
- **Fail-Fast Maintained**: If any required field for a specific binary is missing, the process still crashes instantly at startup. We avoid the false choice between global monolithic validation and weak validation.
- **Modularity**: New background jobs or services can compose exactly the configuration they need without modifying a central file.

### Negative
- Requires a slightly more verbose configuration setup (multiple small structs instead of one large one) in the `internal/config` package.

## Alternatives Considered

### Alternative 1: Single Monolithic Global Configuration Struct
Rejected. A single monolithic configuration forces worker containers (such as ingestion jobs) to require JWT authentication secrets and client-facing settings they do not need, violating least privilege principles.

### Alternative 2: Lazy Runtime Configuration Retrieval via `os.Getenv`
Rejected. Fetching environment variables lazily during request execution defers missing configuration errors to production runtime rather than catching them immediately at application startup.

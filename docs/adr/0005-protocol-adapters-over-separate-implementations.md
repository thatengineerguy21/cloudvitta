# 5. Protocol Adapters Over Separate Implementations

Date: 2026-08-06

## Status

Accepted

## Context

CloudVitta serves price comparisons across two primary interfaces: REST (for web applications and curl) and MCP (for Model Context Protocol AI agent tools).

If each transport implements its own validation, caching orchestration, and matching logic, the system risks business logic divergence and high maintenance costs. We must ensure that core business logic in the calculator engine remains decoupled from the transport layer serving it.

## Decision

We use **Transport Adapters over a Single Core Engine** with strict one-way package dependencies and a centralized domain error interface:
1. **Single Core Engine**: All pricing calculations, attribute matching, and cache coordination reside in `internal/service`. Services accept and return transport-agnostic Go domain types defined in `internal/domain`.
2. **Thin Transport Adapters**: Transport packages (`internal/transport/rest`, `internal/transport/mcp`) function strictly as thin adapters. They deserialize requests, invoke `internal/service`, and serialize domain responses into protocol-specific payloads.
3. **Strict One-Way Imports**: `internal/service` never imports `net/http` or transport-specific packages.
4. **Domain Error Mapping**: `internal/domain` defines typed domain sentinel errors and categories. Transport adapters map domain errors mechanically to protocol-specific status codes (e.g., HTTP 400 with RFC 7807 problem details for REST, or structured MCP error responses).

## Consequences

### Positive
- Bug fixes and pricing logic updates in `internal/service` take effect across REST and MCP simultaneously.
- Core business logic is straightforward to test in isolation without mocking HTTP requests or agent tool invocations.
- Error handling is typed and deterministic across all transport boundaries.

### Negative
- Maintainers must strictly prevent transport concerns (such as HTTP header extraction) from leaking into `internal/service`.
- Adding new error categories requires updating the transport mappers in each transport package.

## Alternatives Considered

### Alternative 1: Independent Business Logic per Transport
Rejected. Writing independent calculation and matching logic per transport causes logic drift, creates inconsistent pricing sums across surfaces, and multiplies test maintenance costs.

### Alternative 2: Monolithic Handlers with Embedded Business Rules
Rejected. Embedding matching and pricing arithmetic inside HTTP handlers tightly couples domain rules to HTTP abstractions and prevents code reuse in MCP tools.

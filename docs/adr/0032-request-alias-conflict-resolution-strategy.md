# 32. Request Alias Conflict Resolution Strategy

Date: 2026-08-23

## Status

Accepted

## Context

In Sub-Stage 3.5, the composite workload calculation endpoint (`POST /api/v1/calculate`) and MCP tool (`calculate_workload`) were updated to accept relational database specifications under both the canonical category key `database_rdbms` and the convenience alias key `database`.

A client could provide values for both keys simultaneously. If the two specifications differ, accepting one key while ignoring the other creates silent divergence, unexpected cost calculations, or security confusion regarding client intent.

## Decision

We introduce a generic alias resolution strategy via `ResolveAliasedField[T any]` and a typed sentinel error `ErrConflictingFields` in `internal/service`:

1. **Resolution Algorithm**:
   - If both `canonical` and `alias` fields are `nil`, return `nil` and no error.
   - If only `canonical` is provided, return `canonical`.
   - If only `alias` is provided, return `alias`.
   - If both `canonical` and `alias` are provided:
     - Perform a deep equality comparison (`reflect.DeepEqual(*canonical, *alias)`).
     - If identical, return `canonical`.
     - If different, return an error wrapping `ErrConflictingFields`.

2. **Transport Error Mapping**:
   - **REST Transport (`internal/transport/rest`)**: Maps `ErrConflictingFields` to HTTP 400 Bad Request with RFC 7807 problem details (`title: "Conflicting Category Fields"`, `detail: "both canonical and alias category specifications were provided with conflicting values"`).
   - **MCP Transport (`internal/transport/mcp`)**: Maps `ErrConflictingFields` to a structured tool execution error.

## Consequences

### Positive
- Prevents silent overrides and enforces clear caller intent.
- Provides consistent, actionable RFC 7807 error responses across all REST and MCP entry points.
- Generic design (`ResolveAliasedField[T any]`) can be reused for any future aliased parameter pairs.

### Negative
- Requires a deep equality check when callers provide both fields.

## Alternatives Considered

### Alternative 1: Canonical Field Silently Overrides Alias Field
Rejected. Silently discarding client-provided alias values violates the Honesty Contract and conceals client-side integration bugs.

### Alternative 2: Alias Field Silently Overrides Canonical Field
Rejected. Unpredictable and prioritizes informal aliases over canonical domain names.

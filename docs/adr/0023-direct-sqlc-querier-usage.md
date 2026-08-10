# 23. Direct sqlc Querier Usage (No Repository Wrapping)

Date: 2026-08-06

## Status

Accepted

## Context

CloudVitta uses `sqlc` to generate type-safe Go code from raw SQL queries. In many web frameworks (especially originating from Node or Java ecosystems), it is common practice to wrap database access in a hand-written "Repository" interface to decouple the business logic from the database implementation and to enable mocking in unit tests.

We needed to decide whether to write a custom Repository interface layer on top of `sqlc`, or to call `sqlc`'s generated types directly from `internal/service`.

## Decision

We explicitly forbid hand-written repository interfaces wrapping `sqlc`. `internal/service` must depend directly on the **`Querier` interface that `sqlc` automatically generates**. 

## Consequences

### Positive
- **No Lost Testability**: `sqlc` natively generates a `Querier` interface. Production wiring passes in the real `*Queries` struct, while unit tests can pass in a mock or a hand-rolled fake satisfying the exact same interface. We achieve fast, isolated, mock-driven unit tests without needing Testcontainers for every test.
- **Zero Boilerplate**: We avoid creating an extra abstraction layer that earns no cost. A hand-written repository interface on top of an already-generated, type-safe data access interface is pure redundant boilerplate.
- **Testing Pyramid Maintained**: We rely on fast mocked unit tests for the service layer, and a smaller, targeted set of integration tests (using Testcontainers) to verify actual SQL semantics and constraints.

### Negative
- Developers accustomed to "Repository Pattern" dogmas must adapt to using the generated `Querier` interface directly.

# 16. Manual Dependency Injection Over DI Frameworks

Date: 2026-08-06

## Status

Accepted

## Context

As CloudVitta's complexity grows to include 7 cloud provider adapters, 4 transport layers (REST, gRPC, MCP, A2A), caches, and FX services, wiring the application dependency graph in `main.go` could become verbose.

We needed to decide whether to use a Dependency Injection (DI) framework. Options included runtime reflection frameworks (e.g., Uber's `fx`) or compile-time code-generation tools (e.g., Google's `wire`), compared against strictly manual, constructor-based DI (e.g., `NewService(repo)`).

## Decision

We will strictly use **Manual, Constructor-based Dependency Injection**, explicitly rejecting DI frameworks like `wire`, `fx`, and `dig`.

## Consequences

### Positive
- **Readability & Debugging**: The entire dependency graph can be understood simply by reading `main.go` (or its helper functions). If a dependency is missing, the standard Go compiler catches it instantly.
- **Avoids Framework Magic**: Runtime frameworks like `fx` defer missing dependency errors to startup time rather than compile time, and debugging initialization failures often requires reading the framework's internal reflection logic instead of our own code.
- **Avoids Codegen Overhead**: While `wire` preserves compile-time safety, the actual dependency graph for CloudVitta is shallow. Seven providers sharing one interface is one wiring pattern repeated; four transports taking one service layer is trivial. The graph is not deep or combinatorial enough to justify the overhead of a codegen step.

### Negative
- `main.go` could become noisy with boilerplate.
- **Mitigation**: We explicitly mitigate this by grouping constructors into small, logical helper functions (e.g., `newProviderAdapters()`, `newCacheLayer()`), keeping the top-level `main.go` clean while remaining 100% compiler-checked and manually traceable.

## Alternatives Considered

### Alternative 1: Runtime Reflection DI Frameworks (Uber `fx`, `dig`)
Rejected. Runtime reflection frameworks hide the initialization order, defer missing dependency failures to runtime execution rather than compilation, and make stack traces difficult to debug during system startup.

### Alternative 2: Compile-Time Code-Generation DI Tools (Google `wire`)
Rejected. Google Wire adds build step complexity and code-generation artifacts for a dependency graph that is shallow and straightforward to construct manually in standard Go.

# 5. Protocol Adapters Over Separate Implementations

Date: 2026-08-06

## Status

Accepted

## Context

CloudVitta is designed to serve price comparisons across multiple protocol surfaces: REST (the primary early demo surface), gRPC (for high-performance internal microservice consumption), MCP (for AI agent tool usage), and eventually A2A. 

If each protocol transport implemented its own parameter validation, caching orchestration, and matching algorithms, we would face severe logic divergence and maintenance overhead. Furthermore, we must ensure that core business logic in the calculator engine remains completely agnostic to the transport layer serving it.

## Decision

We will use **Protocol Adapters over a Single Core Engine** with strict one-way dependencies and a custom domain error interface:
1. **One Engine**: All business logic (matching, caching, FX orchestration) lives in `internal/service`. This service accepts and returns purely protocol-agnostic Go structs defined in `internal/domain`.
2. **Thin Adapters**: The transport packages (`internal/transport/rest`, `internal/transport/grpc`, etc.) act as thin adapters. They do nothing but deserialize protocol-specific requests (e.g., HTTP JSON or Protobuf), call the core engine, and serialize the result.
3. **Strict One-Way Imports**: `internal/service` will *never* import `net/http`, `google.golang.org/grpc`, or any other transport-specific package. 
4. **Domain Error Codes**: To handle errors without leaking transport concerns, `internal/domain` will define a strict custom error interface (`domain.Error`) containing specific internal error codes (e.g., `ErrorCodeValidation`, `ErrorCodeNotFound`, `ErrorCodeRateLimited`). The transport adapters will type-assert these errors and use a `switch` statement to map them definitively to protocol-specific codes (e.g., HTTP `400` vs gRPC `codes.InvalidArgument`).

## Consequences

### Positive
- A bug fixed in the calculator engine automatically fixes it for REST, gRPC, and MCP simultaneously.
- Core business logic is extremely easy to unit test because it does not require mocking HTTP requests or gRPC streams.
- Error handling is strictly typed and deterministic across all surfaces.

### Negative
- Requires discipline to prevent transport logic (like reading an HTTP header) from sneaking into `internal/service`.
- Adding a new error condition requires updating the transport mappers in multiple places.

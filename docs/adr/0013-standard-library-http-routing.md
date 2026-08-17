# 13. Standard Library HTTP Routing over Third-Party Frameworks

Date: 2026-08-06

## Status

Accepted

## Context

For the REST API layer, we needed to decide between using a popular Go web framework (like Gin, Echo, or Fiber) versus relying purely on the Go standard library (`net/http` with the Go 1.22+ enhanced `ServeMux`). 

A common argument for third-party frameworks is that they provide a rich ecosystem of pre-built middleware for JWT validation, rate limiting, and request binding out of the box, whereas the standard library requires handwriting or stitching these together.

## Decision

We will use the **Go Standard Library (`net/http`)** and the 1.22+ `ServeMux`, explicitly rejecting third-party HTTP frameworks.

## Consequences

### Positive
- **Multi-Protocol Reusability**: CloudVitta is a multi-protocol system serving REST, gRPC, MCP, and A2A. A framework's middleware (e.g., Gin's rate limiter) only works for REST. We would have to hand-write equivalent logic for gRPC and MCP regardless. By centralizing cross-cutting concerns (auth, rate limiting) into reusable internal packages that wrap standard Go interfaces, we write the logic once and apply it universally, rather than coupling it to a REST-only framework.
- **No Third-Party API Churn**: We avoid the maintenance burden of tracking a framework's breaking changes across major versions. The standard library provides ultimate stability.
- **Performance**: The standard `ServeMux` (post-1.22) is highly performant and natively supports path parameters and HTTP methods without additional overhead.

### Negative
- Requires a slightly deeper understanding of the standard `func(http.Handler) http.Handler` middleware pattern rather than relying on framework abstractions.

## Alternatives Considered

### Alternative 1: Third-Party HTTP Frameworks (Gin, Echo, Fiber)
Rejected. Framework-specific middlewares (such as Gin rate limiters or auth handlers) cannot be reused across gRPC, MCP, or A2A transports. They introduce third-party API churn and dependency bloat without offering benefits over the standard library `http.ServeMux` (Go 1.22+).

### Alternative 2: Full-Stack Web Application Frameworks
Rejected. Heavy full-stack frameworks introduce opinionated ORM and transport abstractions that contradict our modular monolith architecture and explicit constructor-based dependency injection.

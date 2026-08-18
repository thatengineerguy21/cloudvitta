# 24. Centralized Error Mapping (RFC 7807)

Date: 2026-08-06

## Status

Accepted

## Context

CloudVitta handles requests across transport boundaries (REST and MCP). When the core business logic (`internal/service`) encounters an error (e.g., a validation failure or missing data), that error must be translated into a transport-specific response (e.g., an HTTP 400 with an RFC 7807 JSON payload for REST, or a structured tool error response for MCP).

We needed to decide whether individual endpoint handlers should be responsible for catching errors and formatting the response, or if error translation should be handled centrally by middleware.

## Decision

We mandate **Single Centralized Error Mapping** per transport. Ad-hoc error-to-status-code translations inside individual handlers are strictly forbidden.

1. **Error Categories**: `internal/domain` defines a small, closed set of error categories (e.g., `Validation`, `NotFound`, `RateLimited`).
2. **Context on the Value**: Context-rich details (the "why," specific to a given rule) are attached as structured data to the error value at the point it is raised, not hardcoded into the mapper.
3. **Mechanical Translation**: The centralized middleware purely mechanically reads the category (to set the HTTP status) and the attached details (to populate the RFC 7807 `type`, `title`, `detail`, and `code` fields).

## Consequences

### Positive
- **Guaranteed Consistency**: Every error surfaces the exact same shape (RFC 7807 for REST), ensuring the "honesty vocabulary" (Rule 2) is strictly maintained across the API.
- **No Monolithic Switch Statement**: New business rules simply raise new error values with specific details but belonging to existing broad categories. The centralized mapper never needs to be updated to support a new feature.
- **Clean Handlers**: REST and MCP handlers remain incredibly thin, completely decoupled from error serialization logic.

### Negative
- Requires developers to strictly use the custom domain error constructor rather than standard `fmt.Errorf()` across the entire codebase.

## Alternatives Considered

### Alternative 1: Ad-Hoc Error Formatting Inside Individual Handlers
Rejected. Formatting error responses individually within each REST or MCP handler causes inconsistent status codes, duplicated serialization code, and non-uniform error payload formats.

### Alternative 2: Generic 500 Internal Server Error Responses for Unhandled Errors
Rejected. Returning generic 500 errors obscures validation details and operational guidance, harming client developer experience and violating RFC 7807 problem details standards.

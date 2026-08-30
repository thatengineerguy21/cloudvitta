# 40. Unified Monolith and Embedded Static Asset Serving

Date: 2026-08-30

## Status

Accepted

## Context

CloudVitta previously evaluated two deployment models:
1. **Model A (Two Separate Containers)**: A standalone Go backend container for the REST API and Streamable HTTP MCP server, and a separate Nginx container for the static web frontend.
2. **Model B (Single Unified Container)**: A single Go container that embeds the compiled single-page application (SPA) assets and serves the user interface, REST API, Streamable HTTP MCP server, OpenAPI documentation, and Prometheus telemetry from one origin.

Operating two separate services introduced several operational and technical costs:
- **Cross-Origin Resource Sharing (CORS) Overhead**: Web browser clients had to perform `OPTIONS` preflight network requests before sending credentialed API calls.
- **Third-Party Cookie Restrictions**: The stateless anonymous tracking cookie (`cv_anon_id`) functioned as a third-party cookie across different origins, which modern browsers restrict or block by default.
- **Infrastructure Complexity**: Deploying, routing, and monitoring two separate Cloud Run services increased operational overhead and latency.

## Decision

We adopt **Model B: Single Unified Cloud Run Service** with embedded static asset distribution:

1. **Embedded Static Asset File System (`embed.FS`)**:
   - The Go backend embeds the compiled frontend directory (`web/dist`) into the API binary using Go standard library `//go:embed all:dist`.
   - A dedicated transport package (`internal/transport/spa`) manages static asset delivery and single-page application fallback routing.

2. **Strict API Route Guard**:
   - If an incoming request path matches an API, MCP, documentation, or probe prefix (`/api/`, `/mcp`, `/docs/`, `/healthz`, `/readyz`, `/metrics`), the SPA handler immediately returns an RFC 7807 JSON `404 Not Found` response.
   - The handler never returns HTML for missing or unmapped API endpoints.

3. **HTTP Cache Control and Security Headers**:
   - **Hashed Static Assets (`/assets/*`)**: Served with `Cache-Control: public, max-age=31536000, immutable`.
   - **Entrypoint HTML (`index.html`)**: Served with `Cache-Control: no-cache, no-store, must-revalidate`.
   - **Security Headers**: All responses include `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, and `Referrer-Policy: strict-origin-when-cross-origin`.

4. **First-Party Anonymous Tracking**:
   - Because the frontend and backend share the same origin (`https://cloudvitta.thatengineerguy.in`), the `cv_anon_id` cookie operates as a first-party cookie (`SameSite=Lax`), preventing third-party cookie blocking.

5. **Multi-Stage Containerization (`Dockerfile`)**:
   - Stage 1 (`node:22-alpine`): Compiles TypeScript and builds the production Vite distribution.
   - Stage 2 (`golang:1.26.5-alpine`): Copies the compiled distribution into `internal/transport/spa/dist` and compiles static Go binaries.
   - Stage 3 (`gcr.io/distroless/static-debian12`): Packages minimal non-root runtime container.

## Consequences

### Positive

- **Zero CORS Latency**: Same-origin API calls eliminate HTTP `OPTIONS` preflight requests for the web user interface.
- **Reliable Cookie Isolation**: Anonymous rate-limiting cookies function reliably across all standard web browsers without third-party cookie restrictions.
- **Simplified Operations**: One container image and one Cloud Run service deployment handle the entire application stack.
- **Deterministic API Error Behavior**: API route guards ensure clients and automated agents receive RFC 7807 JSON problem details on 404 errors instead of HTML pages.

### Negative

- Building the production Go binary in Docker requires a Node.js compilation stage prior to Go compilation.
- Changes to static frontend code require rebuilding the binary or running the Vite development server proxy during local development.

## Alternatives Considered

### Alternative 1: Model A Separate Nginx Container
Rejected. Running two separate services creates CORS preflight latency, cookie partitioning issues, and additional deployment management.

### Alternative 2: External Content Delivery Network (CDN) / Object Storage Origin
Rejected. Deploying the frontend to Cloud Storage or a CDN bucket requires multi-resource provisioning and complex DNS routing without solving cookie partitioning for cross-domain API calls.

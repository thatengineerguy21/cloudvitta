# 10. OpenTelemetry with Grafana Cloud

Date: 2026-08-06

## Status

Accepted

## Context

CloudVitta requires robust observability (logs, metrics, and distributed traces) to monitor concurrent ingestion jobs, cache hit rates, and API performance. The original architecture proposed using OpenTelemetry exporting to New Relic, explicitly rejecting the Prometheus/Grafana stack to avoid self-hosting infrastructure.

However, rejecting Prometheus/Grafana conflated two separate questions:
1. Whether to use the Prometheus/Grafana ecosystem (PromQL, Mimir, Loki, Tempo, Grafana dashboards).
2. Whether to self-host that ecosystem.

Self-hosting Prometheus requires a continuously-running process (an always-on VM) to perform interval-based scraping. This directly contradicts the scale-to-zero, serverless economics applied elsewhere in the stack (Neon, Upstash, Cloud Run). The operational burden (patching, storage tuning, securing dashboards) is disproportionate for a solo-maintained portfolio project.

## Decision

We will use **OpenTelemetry exporting to Grafana Cloud**, rather than New Relic or self-hosted Prometheus.

## Consequences

### Positive
- **No Self-Hosted Infrastructure**: Resolves the false tradeoff. Grafana Cloud is a managed version of the open ecosystem (Mimir, Loki, Tempo), eliminating the need for an always-on VM and aligning with our serverless cost constraints.
- **Industry Standard Ecosystem**: Preserves the "industry-standard, not vendor-proprietary" property of PromQL and Grafana, providing a stronger technical signal for a portfolio project than a proprietary query language.
- **Portability**: Because the application uses standard OpenTelemetry instrumentation (not a proprietary agent SDK), migrating to a self-hosted Prometheus stack in the future requires zero application code changes, only an exporter target swap.

### Negative
- Introduces an external SaaS dependency with a free-tier ingest ceiling. This is acceptable at current traffic levels but will require reassessment if usage grows significantly.

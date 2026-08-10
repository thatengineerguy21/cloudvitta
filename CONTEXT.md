# CloudVitta Context

## Overview

**CloudVitta** is a backend service built in Go that normalizes and compares cloud provider workload costs live from official provider APIs (AWS, Azure, GCP, Oracle OCI, IBM Cloud, Alibaba Cloud, DigitalOcean).

## Key Goals & Architecture

1. **Live Pricing Consumption**: Reliable fetching and parsing from official Pricing/Catalog APIs.
2. **Normalization & Currency**: Convert disparate pricing shapes (per-hour, per-GB-month) into comparable units and convert currencies via live FX rates.
3. **Structured Concurrency & Reliability**: Parallel fetches with graceful partial failure using Go goroutines/context.
4. **Caching & API Design**: Serve reads from shared cache; primary demo surface is a documented HTTP API (extending to gRPC, MCP, A2A).


## Architectural Decision Records (ADRs)

Architecture decisions are recorded under [`docs/adr/`](file:///docs/adr/).

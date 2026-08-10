# 15. Decimal Over Float64 for Money

Date: 2026-08-06

## Status

Accepted

## Context

CloudVitta handles pricing data across 7 cloud providers. Prices are ingested, normalized, converted across currencies using FX rates, and summed for composite workload calculations.

We needed to decide whether to represent these monetary values using Go's native, hardware-accelerated `float64` or a software-based arbitrary-precision decimal package (e.g., `github.com/shopspring/decimal`).

A common argument for `float64` is performance, asserting that a caching layer shouldn't incur the heavy computational and memory overhead of software decimals when just passing prices through.

## Decision

We will strictly use **`decimal.Decimal`** for any price amount, FX rate, or computed total, explicitly rejecting `float64` optimizations.

## Consequences

### Positive
- **Guaranteed Accuracy**: CloudVitta is not a pure pass-through system; we sum composite workloads and apply FX conversions. Using `float64` binary rounding (`0.1 + 0.2 = 0.30000000000000004`) would result in totals that do not match manual addition.
- **Prevents Audit Trail Corruption**: Our upsert-with-history model relies on exact comparisons to detect price changes. A floating-point drift would trigger false-positive price changes, polluting the database with redundant historical rows.
- **Preserves Ranking Fidelity**: Exact decimal math prevents ranking flips that could occur due to micro-drifts during FX conversion.

### Negative
- **Minor Overhead**: `decimal.Decimal` carries a computational penalty compared to native floats.
- However, we explicitly accept this because the performance argument is a fallacy for this system: ingestion is entirely I/O-bound, and API requests sum a handful of line items. Optimizing float throughput would be optimizing a part of the system that is fundamentally not the bottleneck. Precision is the core product requirement.

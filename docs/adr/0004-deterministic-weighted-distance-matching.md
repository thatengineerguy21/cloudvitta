# 4. Deterministic Weighted-Distance Matching

Date: 2026-08-06

## Status

Accepted

## Context

Users query CloudVitta by requesting generic workload specifications (e.g., "2 vCPUs, 8GB RAM"). Because cloud providers do not offer identical SKU specifications, CloudVitta uses a deterministic weighted-distance scoring algorithm to find the closest match across AWS, Azure, GCP, and other providers.

However, attribute scoring can result in ties (for example, AWS `t3.large` and AWS `t4g.large` both matching a 2 vCPU / 8GB RAM request). We must decide how to handle ties without returning unpredictable results across identical queries or confusing the user.

## Decision

We use a **Deterministic Tie-Breaker with Surfaced Alternatives**:
1. **Deterministic Primary Winner**: In the event of a tie in the weighted distance score, the engine applies a deterministic secondary tie-breaker (lowest normalized price, followed by alphabetical SKU ID) to select a single primary SKU. This primary SKU determines the top-level price calculation shown for that provider.
2. **Surfaced Alternatives**: To maintain transparency, all tied or near-tied SKUs that lost the tie-breaker are returned in an accompanying `alternatives` array within the API response payload.
3. **Category-Specific Scorers**: Strategy scorers (`CategoryScorer`) implement domain-specific weights and penalty curves (e.g., compute vCPU/RAM distance, storage tier matching, network transfer egress matching).

## Consequences

### Positive
- The API always returns a predictable, deterministic primary price calculation, enabling reliable side-by-side cloud comparisons.
- Advanced users retain full visibility into alternative SKUs without breaking the API contract.
- Simplifies user interfaces by providing a definitive primary choice for summary cards and comparison charts.

### Negative
- API response payload sizes increase slightly due to the inclusion of the `alternatives` array.
- The matching engine must strictly maintain deterministic sorting across all scoring dimensions to guarantee stable caching.

## Alternatives Considered

### Alternative 1: LLM / AI Fuzzy Inference Matching
Rejected. LLM inference introduces non-deterministic outputs across identical requests, adds high latency (hundreds of milliseconds per calculation), incurs recurring token costs, and prevents verifiable mathematical tie-breaking.

### Alternative 2: Static Hardcoded SKU-to-SKU Cross-Reference Tables
Rejected. Hardcoded SKU mapping tables fail to scale as cloud providers introduce new instance families, regions, or hardware revisions, creating continuous manual maintenance bottlenecks.

### Alternative 3: Exact Attribute Filtering Only
Rejected. Strict attribute matching produces frequent false negatives when providers offer slightly mismatched hardware ratios (for example, 7.5 GiB versus 8.0 GiB RAM), unnecessarily rejecting valid comparable instances.

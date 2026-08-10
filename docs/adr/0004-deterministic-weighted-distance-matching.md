# 4. Deterministic Weighted-Distance Matching

Date: 2026-08-06

## Status

Accepted

## Context

Users query CloudVitta by requesting generic workload specifications (e.g., "2 vCPUs, 8GB RAM"). Because cloud providers do not have identical SKU specifications, CloudVitta uses a deterministic weighted-distance scoring algorithm to find the "closest" match across AWS, Azure, GCP, etc. 

However, this scoring mechanism can result in ties (e.g., an AWS `t3.large` and an AWS `t4g.large` both perfectly matching a 2 vCPU / 8GB RAM request). We must decide how to handle ties without overwhelming the user or returning unpredictable results across identical queries.

## Decision

We will use a **Deterministic Tie-Breaker with Surfaced Alternatives**:
1. **Clarity First**: CloudVitta assumes the user is technically literate (Developer/DevOps) but values immediate clarity over chaos.
2. **Primary Winner**: In the event of a tie in the distance score, the engine will apply a deterministic secondary tie-breaker (e.g., lowest price or newest generation) to definitively select a single "Primary SKU". This Primary SKU determines the price calculation shown for that provider.
3. **Surfaced Alternatives**: To maintain transparency and prevent the user from guessing what was omitted, all tied or near-tied SKUs that lost the tie-breaker will be included in an accompanying `alternatives` array within the API payload.

## Consequences

### Positive
- The API always returns a predictable, definitive primary price calculation, making side-by-side cloud comparisons instantly readable.
- Advanced users retain full visibility into alternative SKUs without breaking the API contract.
- Simplifies UI/UX downstream since the frontend only needs to render the primary choice for the immediate comparison graph.

### Negative
- The API response payload size increases slightly due to the inclusion of the `alternatives` array.
- The engine must guarantee that the tie-breaker logic is strictly deterministic, otherwise caching becomes unpredictable.

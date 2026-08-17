# 19. Tiered Pricing in v1

Date: 2026-08-06

## Status

Accepted

## Context

Cloud storage and network egress are almost universally priced using tiers (e.g., first 100GB at $0.10, next 900GB at $0.05). 

Originally, CloudVitta excluded tiered SKUs from v1 normalization to protect the scope, under the assumption that tiered parsing was too complex for an initial release. However, excluding storage and network pricing fundamentally undermines the core value proposition of a "cloud price comparison tool," steering users toward compute-only decisions and producing dishonest blended comparisons.

## Decision

We will treat **Tier-Aware Calculation as a first-class feature in v1**, fully reversing the previous decision to exclude it.

1. **Structured Schema**: Tiered SKUs are normalized into a structured breakpoint schema (`TierSchedule`), not flattened into a scalar price.
2. **Usage Input**: The `/calculate` API endpoint accepts an optional `usage_volume` parameter per line item.
3. **Curated Adapters**: Parsing is handled via curated, per-provider tier adapters rather than a fragile generic parser.
4. **Honesty-First Fallback**: If a specific tiered SKU cannot be parsed, it is explicitly excluded with a `category_not_supported` or `not_yet_ingested` warning, never silently blended.
5. **Incremental Delivery**: We will ship provider-by-provider. A provider without a finished tier adapter simply flags its tiered SKUs as unsupported, while others compare correctly.

## Consequences

### Positive
- The product solves the actual user problem (comparing the *entire* cloud bill, not just compute).
- Avoids silently distorting pricing decisions.
- Follows our core principle that "a wrong number is worse than an honestly missing one," but applies it at a finer, per-SKU grain.

### Negative
- Significantly increases the engineering scope of v1.
- Requires building a more complex calculation engine capable of walking tier breakpoints.
- Modifies the API contract to support usage-volume inputs.

## Alternatives Considered

### Alternative 1: Exclude Storage and Network Categories in Initial Release
Rejected. Omitting tiered storage and network categories forces users to make compute-only decisions and distorts actual multi-cloud infrastructure cost totals.

### Alternative 2: Approximate Flattened Pricing (Fixed Average Price per GB)
Rejected. Flattening tiered schedules into arbitrary single-number averages generates inaccurate cost calculations and violates our core honesty rule that "a wrong number is worse than an honestly missing one."

# 22. Distinct Field Names for Partial Totals

Date: 2026-08-06

## Status

Accepted

## Context

The composite `/calculate` endpoint allows users to request pricing for an entire workload across multiple categories (e.g., Compute, Storage, Network). Sometimes, a provider might have data for Compute and Storage, but be missing Network.

If the API simply adds up the 2 available categories and returns it in the standard `total_normalized_hourly_usd` field alongside a `"partial": true` boolean flag, it creates a dangerous trap. Downstream clients (especially programmatic ones) often run naive sorts on the `total_normalized_hourly_usd` key to find the cheapest provider. A 2-category total will almost always look "cheaper" than a full 3-category total, leading the client to accidentally crown the incomplete provider as the winner.

## Decision

We will **Separate the Field Name** for partial totals, explicitly avoiding overloading a single field with a boolean flag.

If a requested category is missing for a provider, the API must:
1. Include the `"partial": true` flag.
2. **Omit** the `total_normalized_hourly_usd` field completely.
3. Provide the incomplete sum in a distinctly named `partial_total_normalized_hourly_usd` field.

## Consequences

### Positive
- Correct behavior becomes the path of least resistance. A programmatic client executing a naive sort on `total_normalized_hourly_usd` will simply drop or push the partial providers to the bottom/top of the list (depending on the language's null/undefined handling), preventing them from falsely "winning" the cost comparison.
- "Figure it out" is not a safe default for APIs consumed by unknown downstream clients. Separating the field enforces explicit opt-in discipline to compare partial totals against complete ones.

### Negative
- Requires slightly more complex JSON unmarshaling on the client side if they *do* want to render both complete and partial totals in the same UI column.

## Alternatives Considered

### Alternative 1: Overload `total_normalized_hourly_usd` with a Boolean `"partial": true` Flag
Rejected. Naive client sorting logic prioritizes low numerical totals. A two-category sum is almost always cheaper than a three-category sum, causing incomplete provider calculations to be incorrectly ranked as the cheapest option.

### Alternative 2: Fail the Request Entirely When Any Category is Missing
Rejected. Rejecting comparison requests because one provider lacks a specific category eliminates valid cost visibility for users who want to inspect available categories (such as compute and storage) despite missing network data.

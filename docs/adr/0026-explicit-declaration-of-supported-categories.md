# 26. Explicit Declaration of Supported Categories

Date: 2026-08-06

## Status

Accepted

## Context

During early development or beta testing, CloudVitta will inevitably have gaps in provider coverage (e.g., an adapter might support AWS Compute but not yet AWS Storage). 

A common temptation in early-stage products is to return a placeholder value (e.g., a hardcoded approximate average cost) when a category is unsupported, purely to make the UI look complete and polished during demos for stakeholders. We needed to decide whether to permit these "demo placeholders" or enforce a strict honesty policy that explicitly surfaces missing data.

## Decision

We mandate the **Explicit Declaration of Supported Categories** (`supported_categories.go`). If a provider does not support a category, the system must return a `category_not_supported` warning rather than a placeholder value.

## Consequences

### Positive
- **Credibility**: A placeholder doesn't solve the polish problem; it just makes incompleteness invisible, leaving the audience unable to distinguish a real price from a guess. Explicit warnings are a more credible signal of engineering maturity.
- **Proper Separation of Concerns**: The fix for a UI that "looks incomplete" belongs in the frontend, not the data layer. The frontend can render the explicit warning as a polished, intentional state (e.g., "Storage pricing: 4/7 providers live").
- **Prevents Tech Debt**: A placeholder path never stays scoped to "just a demo." It inevitably becomes a silent secondary source of truth that leaks into production. This rule prevents short-term polish from becoming long-term regret.

### Negative
- Requires frontend/client developers to handle `warnings` gracefully in the UI, rather than assuming the `results` array is always perfectly uniform.

## Alternatives Considered

### Alternative 1: Dynamic Discovery of Supported Categories from Database Rows
Rejected. Inferring supported categories solely from database row presence confuses un-ingested or failing categories with permanently unsupported categories, preventing clear operational degradation alerts.

### Alternative 2: Fabricated Placeholder Pricing for Unsupported Categories
Rejected. Returning hardcoded placeholder prices to make comparison matrices look complete during demos corrupts data credibility and leads users to make decisions based on false figures.

# 39. Editorial Bento Design System, Harsh 0px Geometry, and Strict Visual Honesty Contract

Date: 2026-08-30

## Status

Accepted

## Context

Cloud pricing estimation tools commonly use vague rounded cards, imprecise visual metaphors, and silent approximation of missing components. Incomplete totals are frequently presented as full estimates without clear indication that line items are missing.

CloudVitta requires a visual identity and user interface architecture that communicates analytical precision, dense information clarity, and complete adherence to the backend honesty contract (ADR 0004, ADR 0022).

## Decision

1. **Harsh 0px Rectilinear Geometry**:
   - Mandate 0px border radius across all user interface components (`rounded-none`, `borderRadius: 0`).
   - Use high-contrast 1px architectural borders and structural grid layouts (`BentoGrid`, `BentoCard`).

2. **Dual Editorial Theme Tokens**:
   - Provide two semantic themes: *Editorial Bento* (Light Mode) and *Obsidian Editorial* (Dark Mode).
   - Require WCAG AA contrast compliance across all text, borders, and background color tokens.
   - Use typography pairing: *EB Garamond* for display headlines and *Manrope* / *Hanken Grotesk* for technical metrics and table data.

3. **Strict Visual Honesty Contract**:
   - **Zero Client-Side Arithmetic**: The user interface never calculates, converts, or approximates prices in JavaScript. All currency values, totals, and match deltas originate directly from backend API responses.
   - **Explicit Match Quality Tiers**: Render exactly three match quality badges: `EXACT`, `CLOSE`, and `APPROXIMATE`.
   - **Visual Anomaly Flagging**: Display prominent warning indicators (`AnomalyFlag`) when pricing observations have pending review status.
   - **Honesty in Workload Totals (ADR 0022)**: When a composite workload contains missing or failed category estimates, the UI displays a harsh dashed border with an explicit `[PARTIAL]` indicator and omits complete total metrics.

## Consequences

### Positive

- **Transparent Pricing Data**: Users can immediately identify exact matches, approximations, anomalies, and incomplete multi-category totals.
- **Distinct Visual Identity**: 0px rectilinear geometry and editorial typography create an analytical, publication-grade user experience.
- **WCAG AA Compliance**: High-contrast tokens ensure accessibility across both light and dark display modes.

### Negative

- Strict 0px geometry requires custom form and card components rather than standard third-party rounded component libraries.

## Alternatives Considered

### Alternative 1: Standard Rounded Modern SaaS Component Libraries
Rejected. Generic rounded cards lack the analytical precision and distinct aesthetic required for CloudVitta.

### Alternative 2: Client-Side Fallback Pricing Calculations
Rejected. Performing mathematical estimations in the browser violates the single source of truth rule and risks displaying inaccurate data.

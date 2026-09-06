# 42. Soft Warm Modern and Obsidian Terracotta Bento Redesign

Date: 2026-09-06

## Status

Accepted (Supersedes geometry constraints of ADR 0039; preserves visual honesty contract)

## Context

ADR 0039 defined an Editorial Bento design system with harsh 0px rectilinear geometry. The 0px borders delivered analytical precision. However, user feedback indicated that the interface felt excessively industrial and cold. Furthermore, dense pricing tables made fast cross-provider comparison difficult on small viewports and modern mobile screens.

CloudVitta requires a refined user experience that provides soft visual warmth, modern spatial hierarchy, and clear comparative highlights, while strictly keeping all backend honesty guarantees from ADR 0004, ADR 0022, and ADR 0039.

## Decision

1. **Evolution to Soft Warm Modern and Obsidian Terracotta Themes**:
   - **Soft Warm Modern (Light Mode)**:
     - Canvas Foundation: Warm Alabaster (`#FAFAF8`).
     - Elevated Cards: Pure White (`#FFFFFF`).
     - Subordinate Wells and Inputs: Soft Stone (`#F5F4F0`).
     - Brand Accent: Burnt Sienna (`#E06D53`).
     - Success / Best-Value Indicator: Forest Emerald (`#059669`).
   - **Obsidian Terracotta (Dark Mode)**:
     - Foundation Canvas: Obsidian (`#111316`).
     - Elevated Cards: Slate Charcoal (`#181B20`).
     - Subordinate Wells: Deep Charcoal (`#1E2023`).
     - Brand Accent: Warm Terracotta (`#E06D53`).
     - Success Indicator: Soft Emerald (`#2A9D8F`).

2. **Soft Rounded Bento Geometry Hierarchy**:
   - Remove the global 0px border-radius override (`* { border-radius: 0px !important; }`).
   - Establish an intentional rounded scale:
     - Outer cards, modal containers, and hero sections: `rounded-2xl` (16px) with soft ambient box shadows (`shadow-sm`, `shadow-md`, `shadow-2xl`).
     - Form inputs, spec wells, dropdowns, and buttons: `rounded-xl` (12px).
     - Checkboxes and compact item containers: `rounded-lg` (8px) or `rounded` (4px).
     - Telemetry badges, match quality tags, and status pills: `rounded-full` (9999px).

3. **Typography Pairing**:
   - Technical & Navigation Sans: *Plus Jakarta Sans* and *Inter* for legible metrics, form labels, and compact technical specs.
   - Editorial Wordmark & Headings: *Manrope* and *EB Garamond* for display headlines, brand identity, and section titles.
   - Code & Pricing Monospace: *JetBrains Mono* with `.tabular-nums` formatting to prevent numeral jitter during live sorting.

4. **Comparison Architecture**:
   - **4-Column Responsive Bento Cards Grid**: Default comparison presentation on all service pages using `ProviderCompareCard`. Each card features:
     - Prominent "Lowest TCO" emerald ring and badge for the price floor.
     - 2x2 specification pill grid (vCPU, Memory, Network, Silicon/Arch).
     - Large normalized pricing with savings percentage delta and approximate monthly cost.
     - Raw SKU catalog JSON modal dialog for complete developer auditability.
   - **4-Metric Highlights Bento Row**: Renders Arbitrage Floor, Median Market Price, Annual Maximum Spread, and Architecture Distribution across all matched offerings.
   - **Dual View Toggle**: Retains the dense HTML table view accessible via an instant `[ Cards Grid | Table View ]` toggle switch.
   - **Honesty Contract Banner**: Dedicated header banner highlighting deterministic calculation and direct OpenAPI audit paths.

5. **Strict Adherence to Visual Honesty Contract**:
   - **Zero Client-Side Arithmetic**: The user interface does not perform price calculations, conversions, or rounding. All values originate from backend SQLC queries.
   - **Partial Workload Indication**: Workloads with missing components retain dashed borders and explicit partial estimate warnings.
   - **Anomaly Flagging**: Observations with pending review status display warning badges.

## Consequences

### Positive

- **Enhanced Visual Ergonomics**: Soft warm modern tones and rounded corners reduce eye fatigue and make information dense dashboards approachable.
- **Improved Information Hierarchy**: Bento metric highlights and responsive provider cards allow users to identify optimal cloud options in seconds.
- **Full Backward Compatibility**: All 43 test suites and 183 unit tests remain 100% passing because data attributes and test IDs (`data-testid="results-table"`, `row-[provider]`) are fully preserved.
- **Uncompromised Truthfulness**: Deterministic calculation and honesty contract guarantees remain non-negotiable.

### Negative

- Tailwind configuration requires maintenance of custom border-radius and shadow token sets.

## Alternatives Considered

### Alternative 1: Retain Pure 0px Rectilinear Geometry
Rejected. Harsh 0px borders caused visual rigidity and reduced visual distinction between primary cards and secondary wells.

### Alternative 2: Replace Grid Cards with Accordion-Only Rows
Rejected. Horizontal cards grid optimizes wide screens and supports simultaneous side-by-side spec comparison across multiple clouds.

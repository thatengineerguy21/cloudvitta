# 31. Registry Architecture for Extensible Service Categories

Date: 2026-08-23

## Status

Accepted

## Context

In Stage 0 and Stage 1, CloudVitta supported three service categories (`compute`, `storage`, `network`). Attribute serialization, pricing arithmetic, and composite calculation dispatch used explicit `switch` statements across these three categories.

In Stage 3, the service catalog expanded to seven categories (`compute`, `storage`, `network`, `database_rdbms`, `database_nosql`, `kubernetes`, `serverless`). Using monolithic `switch` statements across domain serialization, service matching, calculate request parsing, and pricing arithmetic introduced tight coupling, boilerplate duplication, and high regression risk whenever a new category was introduced.

## Decision

We introduce category-keyed strategy registries across domain and service layers:

1. **Domain Attribute Codec Registry (`internal/domain/price_observation.go`)**:
   - `RegisterCategoryAttributeCodec(category, codec)` registers category-specific JSONB marshaler and unmarshaler functions.
   - `PriceObservation.MarshalAttributes()` and `PriceObservation.UnmarshalAttributes()` dynamically dispatch serialization to the registered codec.

2. **Calculate Category Registry (`internal/service/calculate.go`)**:
   - `RegisterCalculateCategory(category, evaluator)` registers category extraction and `MatchTarget` construction strategies.
   - `PricingService.Calculate()` iterates through the registered calculate categories, inspecting request payloads and assembling match targets without hardcoded category branches.

3. **Pricing Arithmetic Registry (`internal/service/pricing_calc.go`)**:
   - `RegisterCategoryPricingHandler(category, handler)` registers category-specific hourly and monthly pricing functions.
   - `CalculateHourlyCost()` and `CalculateMonthlyCost()` delegate to the registered strategy for the given category.

## Consequences

### Positive
- Adheres to the Open/Closed Principle: new service categories can register codecs, scorers, and calculators without modifying existing core loops.
- Eliminates repeated, fragile 7-category `switch` statements.
- Simplifies testing of individual category serializers and pricing handlers in isolation.

### Negative
- Codecs and handlers must execute registration during package initialization (`init()`) or server startup.
- Registry lookups add negligible in-memory map lookup operations during request processing.

## Alternatives Considered

### Alternative 1: Monolithic Switch Blocks in Core Services
Rejected. Adding each new category required modifying every switch statement across the codebase, increasing merge conflicts and risk of missed branches.

### Alternative 2: Reflection-Based Dynamic Struct Mapping
Rejected. Reflection adds runtime overhead, bypasses compile-time type safety, and complicates static code analysis.

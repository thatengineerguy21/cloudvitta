# 27. Use koanf and validator for Configuration

Date: 2026-08-11

## Status

Accepted

## Context

Originally, the project relied on standard library `os.Getenv` calls and manual validation for configuration (ADR 25). However, this led to significant repetition, especially when prefixes like `CLOUDVITTA_` were introduced. Every environment variable required manual fetching, prefixing, type conversion, and empty-string checking, which would become increasingly tedious and error-prone as the configuration surface grew in Stages 1 and 2.

## Decision

We will use `github.com/knadh/koanf/v2` combined with `github.com/go-playground/validator/v10` for configuration loading and validation.

1. **`koanf`**: Handles unmarshaling environment variables directly into a nested Go struct based on struct tags. It handles the `CLOUDVITTA_` prefix stripping uniformly.
2. **`validator/v10`**: Provides robust, struct-tag based validation (e.g., `validate:"required"`) to ensure the fail-fast principle is strictly upheld without manual `if field == ""` blocks.
3. **`godotenv/autoload`**: Automatically handles loading variables from `.env` files into the process environment for local development convenience.

## Consequences

### Positive
- **DRY (Don't Repeat Yourself)**: Eliminates boilerplate string manipulation and repetitive manual validation checks.
- **Scalability**: Adding new configuration fields in the future is as simple as adding a new struct field with the appropriate tags.
- **Consistency**: Centralizes prefix handling and standardizes validation logic across the entire application.

### Negative
- **Additional Dependencies**: Adds third-party dependencies (`koanf`, `validator`, `godotenv`) to the project for configuration management, slightly increasing the binary size and dependency graph complexity.

## Alternatives Considered

### Alternative 1: Manual `os.Getenv` Calls and Repetitive Validation Checks
Rejected. Hand-writing environment variable lookups, type parsing, and non-empty string checks produces hundreds of lines of boilerplate that is error-prone to maintain across dozens of configuration fields.

### Alternative 2: Heavy Configuration Frameworks (Viper)
Rejected. Viper relies heavily on global state, possesses a large transitive dependency graph, and introduces complex key casing behaviors compared to the lightweight, modular architecture of `koanf`.

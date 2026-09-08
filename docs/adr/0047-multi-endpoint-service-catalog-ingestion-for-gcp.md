# ADR 0047: Multi-Endpoint Service Catalog Ingestion for Google Cloud Platform

## Context
Google Cloud Platform (GCP) does not publish all database and compute products under single service catalog endpoints.
In the Google Cloud Billing Catalog API, related products publish under distinct service identifiers:
- Relational databases publish under both Cloud SQL (`9662-B51E-5089`) and AlloyDB for PostgreSQL (`C49F-B7F2-7416`).
- NoSQL databases publish under both Cloud Firestore (`EE2C-7FAC-5E08`) and Cloud Bigtable (`C3BE-24A5-0975`).
- Serverless compute products publish under both Cloud Run Functions (`29E7-DA93-CA13`) and Cloud Run container services (`152E-C115-5142`).

Before this change, the GCP client configured only one single URL per category.
Consequently, AlloyDB, Cloud Bigtable, and container-based Cloud Run services were absent from ingestion.

## Decision
1. **Multi-Endpoint URL Configuration**:
   Extend the GCP `Client` to support multiple catalog URLs via `WithURLs(urls ...string)`.
   Provide `CategoryServiceIDs(category string) []string` and `CategoryBillingCatalogURLs(category string) []string` to return all service endpoints for each category.

2. **Sequential Endpoint Fetching in Adapter**:
   Update `Adapter.Fetch` to iterate across all configured URLs.
   Stream each page of each endpoint to raw storage with a unique key format (`raw/gcp/{category}/{date}/{fetchID}-ep{epIdx}-page{pageIdx}.json`).

3. **Taxonomy and Normalizer Mapping**:
   Register service IDs in `internal/matching/catalogmap/gcp.go`.
   In `normalizeDatabaseNoSQLSKU`, map Bigtable nodes to `ComponentType = "throughput"`, `PricingMode = "provisioned"`, `DataModel = "wide_column"`, and `Unit = "Hrs"`. Map Bigtable storage to `ComponentType = "storage"`.
   In `serverlessarchmap/gcp.go`, map Cloud Run container and request metrics to canonical architecture `x86_64`.
   In `NormalizeForCategory`, add fallback extraction of service identifiers embedded in `sku.Name` paths (`services/{serviceId}/skus/...`).

## Consequences
- The GCP adapter captures complete catalog pricing across multi-endpoint services without dropping related products.
- Raw storage persists payloads from all endpoints for complete audit trails.
- Backward compatibility is preserved for single-endpoint configurations and test suites.

-- 0006_cleanup_category_contamination_and_duplicates.sql
-- Strategy: Forward-Only Migration (No rollback/down migration script, ADR 0028).
-- Purpose: Purges misclassified Data Transfer records from the compute category and
--          deduplicates historical zero-dollar ($0.00) observations while preserving the latest last_seen_at.

-- Step 1: Purge misclassified Data Transfer records erroneously stored under service_category = 'compute'
DELETE FROM price_observations
WHERE service_category = 'compute'
  AND (
      display_name ILIKE '%Data Transfer%'
      OR sku_id IN ('MQRM9JPA873FD2UM', 'UG5MQPCK8Z79YJK4', 'MY9NDH3WXB8MKNYA', 'MG9PRVQZCTDERTUT', 'VSVTR2M82RS49DV2')
  );

-- Step 2: Bump last_seen_at on keeper records for duplicate $0.00 observations
WITH duplicates AS (
    SELECT
        provider,
        service_category,
        sku_id,
        region,
        unit,
        pricing_model,
        MIN(id) AS keeper_id,
        MAX(last_seen_at) AS max_last_seen
    FROM price_observations
    WHERE price_amount = 0
    GROUP BY provider, service_category, sku_id, region, unit, pricing_model
    HAVING COUNT(*) > 1
)
UPDATE price_observations p
SET last_seen_at = d.max_last_seen
FROM duplicates d
WHERE p.id = d.keeper_id
  AND p.last_seen_at < d.max_last_seen;

-- Step 3: Remove duplicate $0.00 observations, retaining the earliest record (keeper_id)
WITH duplicates AS (
    SELECT
        provider,
        service_category,
        sku_id,
        region,
        unit,
        pricing_model,
        MIN(id) AS keeper_id
    FROM price_observations
    WHERE price_amount = 0
    GROUP BY provider, service_category, sku_id, region, unit, pricing_model
    HAVING COUNT(*) > 1
)
DELETE FROM price_observations p
USING duplicates d
WHERE p.provider = d.provider
  AND p.service_category = d.service_category
  AND p.sku_id = d.sku_id
  AND p.region = d.region
  AND p.unit = d.unit
  AND p.pricing_model = d.pricing_model
  AND p.price_amount = 0
  AND p.id > d.keeper_id;

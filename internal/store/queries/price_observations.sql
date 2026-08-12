-- name: InsertPriceObservation :one
INSERT INTO price_observations (
    provider,
    service_category,
    sku_id,
    display_name,
    region,
    region_group,
    unit,
    price_amount,
    price_currency,
    pricing_model,
    attributes,
    raw_response_ref,
    fetched_at,
    last_seen_at,
    anomaly_status
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15
)
RETURNING id;

-- name: GetPriceObservations :many
SELECT
    id,
    provider,
    service_category,
    sku_id,
    display_name,
    region,
    region_group,
    unit,
    price_amount,
    price_currency,
    pricing_model,
    attributes,
    raw_response_ref,
    fetched_at,
    last_seen_at,
    anomaly_status
FROM price_observations
WHERE provider = $1
  AND service_category = $2
  AND region_group = $3
ORDER BY fetched_at DESC;

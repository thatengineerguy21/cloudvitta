-- name: GetProviderCategoryStatus :many
SELECT
    service_category,
    MAX(fetched_at)::timestamptz AS last_fetched_at,
    MAX(last_seen_at)::timestamptz AS last_seen_at,
    COUNT(*)::bigint AS observation_count
FROM price_observations
WHERE provider = $1
GROUP BY service_category;

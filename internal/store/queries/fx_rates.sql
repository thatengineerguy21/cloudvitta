-- name: UpsertFXRate :one
INSERT INTO fx_rates (
    base_currency,
    target_currency,
    rate,
    source,
    rate_date,
    fetched_at,
    created_at
) VALUES (
    $1, $2, $3, $4, $5, $6, now()
)
ON CONFLICT (base_currency, target_currency, rate_date)
DO UPDATE SET
    rate = EXCLUDED.rate,
    source = EXCLUDED.source,
    fetched_at = EXCLUDED.fetched_at
RETURNING id, base_currency, target_currency, rate, source, rate_date, fetched_at, created_at;

-- name: GetLatestFXRate :one
SELECT id, base_currency, target_currency, rate, source, rate_date, fetched_at, created_at
FROM fx_rates
WHERE base_currency = $1 AND target_currency = $2
ORDER BY rate_date DESC, fetched_at DESC
LIMIT 1;

-- name: ListLatestFXRates :many
SELECT DISTINCT ON (base_currency, target_currency)
    id, base_currency, target_currency, rate, source, rate_date, fetched_at, created_at
FROM fx_rates
WHERE base_currency = $1
ORDER BY base_currency, target_currency, rate_date DESC, fetched_at DESC;

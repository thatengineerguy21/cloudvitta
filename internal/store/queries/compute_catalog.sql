-- name: UpsertComputeCatalogItem :one
INSERT INTO compute_instance_catalog (
    provider,
    instance_type_id,
    display_name,
    instance_family,
    category,
    vcpu,
    memory_gib,
    cpu_architecture,
    gpu_count,
    gpu_type,
    is_burstable,
    is_current_gen,
    first_seen_at,
    last_seen_at,
    attributes
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15
)
ON CONFLICT (provider, instance_type_id)
DO UPDATE SET
    display_name = EXCLUDED.display_name,
    instance_family = EXCLUDED.instance_family,
    category = EXCLUDED.category,
    vcpu = EXCLUDED.vcpu,
    memory_gib = EXCLUDED.memory_gib,
    cpu_architecture = EXCLUDED.cpu_architecture,
    gpu_count = EXCLUDED.gpu_count,
    gpu_type = EXCLUDED.gpu_type,
    is_burstable = EXCLUDED.is_burstable,
    is_current_gen = EXCLUDED.is_current_gen,
    last_seen_at = EXCLUDED.last_seen_at,
    attributes = EXCLUDED.attributes
RETURNING id;

-- name: GetComputeCatalogSummary :many
SELECT
    provider,
    category,
    COUNT(*)::bigint AS instance_count
FROM compute_instance_catalog
GROUP BY provider, category
ORDER BY provider, category;

-- name: GetTotalComputeInstanceCountByProvider :many
SELECT
    provider,
    COUNT(*)::bigint AS total_instances
FROM compute_instance_catalog
GROUP BY provider
ORDER BY total_instances DESC;

-- name: ListComputeCatalogItems :many
SELECT
    id,
    provider,
    instance_type_id,
    display_name,
    instance_family,
    category,
    vcpu,
    memory_gib,
    cpu_architecture,
    gpu_count,
    gpu_type,
    is_burstable,
    is_current_gen,
    first_seen_at,
    last_seen_at,
    attributes
FROM compute_instance_catalog
WHERE (sqlc.narg('provider')::text IS NULL OR provider = sqlc.narg('provider'))
  AND (sqlc.narg('category')::text IS NULL OR category = sqlc.narg('category'))
  AND (sqlc.narg('instance_family')::text IS NULL OR instance_family = sqlc.narg('instance_family'))
  AND (sqlc.narg('min_vcpu')::numeric IS NULL OR vcpu >= sqlc.narg('min_vcpu'))
  AND (sqlc.narg('max_vcpu')::numeric IS NULL OR vcpu <= sqlc.narg('max_vcpu'))
  AND (sqlc.narg('min_memory_gib')::numeric IS NULL OR memory_gib >= sqlc.narg('min_memory_gib'))
  AND (sqlc.narg('max_memory_gib')::numeric IS NULL OR memory_gib <= sqlc.narg('max_memory_gib'))
ORDER BY provider, category, vcpu, memory_gib
LIMIT $1 OFFSET $2;

-- name: CountComputeCatalogItems :one
SELECT COUNT(*)::bigint
FROM compute_instance_catalog
WHERE (sqlc.narg('provider')::text IS NULL OR provider = sqlc.narg('provider'))
  AND (sqlc.narg('category')::text IS NULL OR category = sqlc.narg('category'))
  AND (sqlc.narg('instance_family')::text IS NULL OR instance_family = sqlc.narg('instance_family'))
  AND (sqlc.narg('min_vcpu')::numeric IS NULL OR vcpu >= sqlc.narg('min_vcpu'))
  AND (sqlc.narg('max_vcpu')::numeric IS NULL OR vcpu <= sqlc.narg('max_vcpu'))
  AND (sqlc.narg('min_memory_gib')::numeric IS NULL OR memory_gib >= sqlc.narg('min_memory_gib'))
  AND (sqlc.narg('max_memory_gib')::numeric IS NULL OR memory_gib <= sqlc.narg('max_memory_gib'));

-- name: GetComputeCatalogItem :one
SELECT
    id,
    provider,
    instance_type_id,
    display_name,
    instance_family,
    category,
    vcpu,
    memory_gib,
    cpu_architecture,
    gpu_count,
    gpu_type,
    is_burstable,
    is_current_gen,
    first_seen_at,
    last_seen_at,
    attributes
FROM compute_instance_catalog
WHERE provider = $1 AND instance_type_id = $2
LIMIT 1;

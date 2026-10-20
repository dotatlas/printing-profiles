-- Read-only catalog queries. Each list returns rows in slug order after a cursor (keyset pagination).
-- Every pattern in "patterns" must match (ILIKE ALL), so each search word must appear somewhere in the text.
-- The material color filter is one ILIKE pattern, or an empty text for all colors. "colors" is a JSON list of {name, hex}.

-- name: ListPrinters :many
SELECT p.slug, p.model, b.slug AS brand_slug, b.name AS brand_name
FROM printers p
JOIN brands b ON b.id = p.brand_id
WHERE (b.name || ' ' || p.model) ILIKE ALL (sqlc.arg(patterns)::text[])
  AND p.slug > sqlc.arg(after)::text
ORDER BY p.slug
LIMIT sqlc.arg(max_rows)::int;

-- name: ListMaterials :many
SELECT m.slug, m.product_name, m.type, m.diameter_mm::float8 AS diameter_mm, b.slug AS brand_slug, b.name AS brand_name,
       (SELECT coalesce(json_agg(json_build_object('name', c.name, 'hex', c.hex) ORDER BY c.name), '[]')
        FROM material_colors c WHERE c.material_id = m.id)::json AS colors
FROM materials m
JOIN brands b ON b.id = m.brand_id
WHERE (b.name || ' ' || m.product_name || ' ' || m.type) ILIKE ALL (sqlc.arg(patterns)::text[])
  AND (sqlc.arg(type)::text = '' OR m.type = sqlc.arg(type)::text)
  AND (sqlc.arg(color)::text = '' OR EXISTS (
        SELECT 1 FROM material_colors c WHERE c.material_id = m.id AND c.name ILIKE sqlc.arg(color)::text))
  AND m.slug > sqlc.arg(after)::text
ORDER BY m.slug
LIMIT sqlc.arg(max_rows)::int;

-- name: ListSlicers :many
SELECT slug, name
FROM slicers
WHERE name ILIKE ALL (sqlc.arg(patterns)::text[])
  AND slug > sqlc.arg(after)::text
ORDER BY slug
LIMIT sqlc.arg(max_rows)::int;

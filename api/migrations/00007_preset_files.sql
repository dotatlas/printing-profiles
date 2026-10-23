-- +goose Up
-- A preset can carry one checked preset file, and counts its downloads. The view adds the file's fields at the end.

ALTER TABLE presets ADD COLUMN file_upload_id bigint REFERENCES uploads (id);
ALTER TABLE presets ADD COLUMN download_count bigint NOT NULL DEFAULT 0;

DROP VIEW preset_view;
CREATE VIEW preset_view AS
SELECT p.id, p.public_id, p.author_id,
       coalesce(u.handle, '')::text AS author_handle, u.display_name AS author_display_name,
       p.title, p.slug, p.description,
       pr.slug AS printer_slug, pr.model AS printer_model, pb.slug AS printer_brand_slug, pb.name AS printer_brand_name,
       m.slug AS material_slug, m.product_name AS material_product_name, m.type AS material_type,
       mb.slug AS material_brand_slug, mb.name AS material_brand_name,
       p.nozzle_mm::float8 AS nozzle_mm,
       s.slug AS slicer_slug, s.name AS slicer_name, p.slicer_version,
       p.license, p.status, p.created_at, p.updated_at, p.published_at, p.deleted_at,
       coalesce(f.file_key, '')::text AS file_key, coalesce(f.slicer, '')::text AS file_slicer,
       coalesce(f.file_kind, '')::text AS file_kind, coalesce(f.slicer_version, '')::text AS file_slicer_version,
       coalesce(f.settings, '[]'::jsonb) AS file_settings, p.download_count
FROM presets p
JOIN users u ON u.id = p.author_id
JOIN printers pr ON pr.id = p.printer_id
JOIN brands pb ON pb.id = pr.brand_id
JOIN materials m ON m.id = p.material_id
JOIN brands mb ON mb.id = m.brand_id
JOIN slicers s ON s.id = p.slicer_id
LEFT JOIN uploads f ON f.id = p.file_upload_id;

-- +goose Down
DROP VIEW preset_view;
CREATE VIEW preset_view AS
SELECT p.id, p.public_id, p.author_id,
       coalesce(u.handle, '')::text AS author_handle, u.display_name AS author_display_name,
       p.title, p.slug, p.description,
       pr.slug AS printer_slug, pr.model AS printer_model, pb.slug AS printer_brand_slug, pb.name AS printer_brand_name,
       m.slug AS material_slug, m.product_name AS material_product_name, m.type AS material_type,
       mb.slug AS material_brand_slug, mb.name AS material_brand_name,
       p.nozzle_mm::float8 AS nozzle_mm,
       s.slug AS slicer_slug, s.name AS slicer_name, p.slicer_version,
       p.license, p.status, p.created_at, p.updated_at, p.published_at, p.deleted_at
FROM presets p
JOIN users u ON u.id = p.author_id
JOIN printers pr ON pr.id = p.printer_id
JOIN brands pb ON pb.id = pr.brand_id
JOIN materials m ON m.id = p.material_id
JOIN brands mb ON mb.id = m.brand_id
JOIN slicers s ON s.id = p.slicer_id;

ALTER TABLE presets DROP COLUMN download_count;
ALTER TABLE presets DROP COLUMN file_upload_id;

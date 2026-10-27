-- +goose Up
-- Result photos: the author of a preset adds checked photos of a print. A trigger keeps the preset's photo count in
-- the same transaction, so "Photo verified" and its filter read one column.

CREATE TABLE photos (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id  text NOT NULL UNIQUE CHECK (public_id ~ '^[a-z2-9]{12}$'),
    preset_id  bigint NOT NULL REFERENCES presets (id),
    upload_id  bigint NOT NULL UNIQUE REFERENCES uploads (id),
    alt        text NOT NULL CHECK (length(alt) BETWEEN 1 AND 200),
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX photos_preset ON photos (preset_id, created_at, id);

ALTER TABLE presets ADD COLUMN photo_count integer NOT NULL DEFAULT 0;

-- +goose StatementBegin
CREATE FUNCTION photos_count() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        UPDATE presets SET photo_count = photo_count - 1 WHERE id = OLD.preset_id;
    ELSE
        UPDATE presets SET photo_count = photo_count + 1 WHERE id = NEW.preset_id;
    END IF;
    RETURN NULL;
END
$$;
-- +goose StatementEnd
CREATE TRIGGER photos_count AFTER INSERT OR DELETE ON photos FOR EACH ROW EXECUTE FUNCTION photos_count();

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
       coalesce(f.settings, '[]'::jsonb) AS file_settings, p.download_count,
       p.rating_count, p.rating_sum, p.photo_count
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
       p.license, p.status, p.created_at, p.updated_at, p.published_at, p.deleted_at,
       coalesce(f.file_key, '')::text AS file_key, coalesce(f.slicer, '')::text AS file_slicer,
       coalesce(f.file_kind, '')::text AS file_kind, coalesce(f.slicer_version, '')::text AS file_slicer_version,
       coalesce(f.settings, '[]'::jsonb) AS file_settings, p.download_count,
       p.rating_count, p.rating_sum
FROM presets p
JOIN users u ON u.id = p.author_id
JOIN printers pr ON pr.id = p.printer_id
JOIN brands pb ON pb.id = pr.brand_id
JOIN materials m ON m.id = p.material_id
JOIN brands mb ON mb.id = m.brand_id
JOIN slicers s ON s.id = p.slicer_id
LEFT JOIN uploads f ON f.id = p.file_upload_id;

DROP TRIGGER photos_count ON photos;
DROP FUNCTION photos_count();
ALTER TABLE presets DROP COLUMN photo_count;
DROP TABLE photos;

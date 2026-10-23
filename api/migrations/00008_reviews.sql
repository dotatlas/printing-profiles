-- +goose Up
-- Reviews: 1-5 stars and a text, one per member per preset. A trigger keeps the preset's rating totals in the same
-- transaction as every review change, so the totals always match the reviews.

CREATE TABLE reviews (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    preset_id  bigint NOT NULL REFERENCES presets (id),
    author_id  bigint NOT NULL REFERENCES users (id),
    stars      smallint NOT NULL CHECK (stars BETWEEN 1 AND 5),
    body       text NOT NULL DEFAULT '' CHECK (length(body) <= 2000),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (preset_id, author_id)
);
CREATE INDEX reviews_preset ON reviews (preset_id, created_at DESC, id DESC);
CREATE INDEX reviews_author ON reviews (author_id);

ALTER TABLE presets ADD COLUMN rating_count integer NOT NULL DEFAULT 0;
ALTER TABLE presets ADD COLUMN rating_sum integer NOT NULL DEFAULT 0;

-- +goose StatementBegin
CREATE FUNCTION reviews_totals() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP IN ('UPDATE', 'DELETE') THEN
        UPDATE presets SET rating_count = rating_count - 1, rating_sum = rating_sum - OLD.stars WHERE id = OLD.preset_id;
    END IF;
    IF TG_OP IN ('INSERT', 'UPDATE') THEN
        UPDATE presets SET rating_count = rating_count + 1, rating_sum = rating_sum + NEW.stars WHERE id = NEW.preset_id;
    END IF;
    RETURN NULL;
END
$$;
-- +goose StatementEnd
-- Only a change of the stars (or the preset) changes the totals; an edit of the text does not.
CREATE TRIGGER reviews_totals AFTER INSERT OR DELETE OR UPDATE OF stars, preset_id ON reviews
    FOR EACH ROW EXECUTE FUNCTION reviews_totals();

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
       coalesce(f.settings, '[]'::jsonb) AS file_settings, p.download_count
FROM presets p
JOIN users u ON u.id = p.author_id
JOIN printers pr ON pr.id = p.printer_id
JOIN brands pb ON pb.id = pr.brand_id
JOIN materials m ON m.id = p.material_id
JOIN brands mb ON mb.id = m.brand_id
JOIN slicers s ON s.id = p.slicer_id
LEFT JOIN uploads f ON f.id = p.file_upload_id;

DROP TRIGGER reviews_totals ON reviews;
DROP FUNCTION reviews_totals();
ALTER TABLE presets DROP COLUMN rating_sum;
ALTER TABLE presets DROP COLUMN rating_count;
DROP TABLE reviews;

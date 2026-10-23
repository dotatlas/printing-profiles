-- +goose Up
-- Presets: the settings for one printer + material + nozzle + slicer. Files, settings and photos come later.

CREATE TABLE presets (
    id             bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id      text NOT NULL UNIQUE CHECK (public_id ~ '^[a-z2-9]{8}$'),
    author_id      bigint NOT NULL REFERENCES users (id),
    title          text NOT NULL CHECK (length(title) BETWEEN 5 AND 100),
    slug           text NOT NULL CHECK (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$' AND length(slug) <= 60),
    description    text NOT NULL DEFAULT '' CHECK (length(description) <= 4000),
    printer_id     bigint NOT NULL REFERENCES printers (id),
    material_id    bigint NOT NULL REFERENCES materials (id),
    nozzle_mm      numeric(3, 2) NOT NULL CHECK (nozzle_mm IN (0.2, 0.25, 0.3, 0.4, 0.5, 0.6, 0.8, 1.0, 1.2)),
    slicer_id      bigint NOT NULL REFERENCES slicers (id),
    slicer_version text NOT NULL DEFAULT '' CHECK (length(slicer_version) <= 20),
    license        text NOT NULL DEFAULT 'CC-BY-4.0' CHECK (license IN ('CC-BY-4.0', 'CC-BY-SA-4.0', 'CC-BY-NC-4.0', 'CC0-1.0')),
    -- hidden is for moderation (later). A deleted preset keeps its row, so reviews and reports keep their target.
    status         text NOT NULL CHECK (status IN ('draft', 'published', 'hidden')),
    created_at     timestamptz NOT NULL DEFAULT now(),
    updated_at     timestamptz NOT NULL DEFAULT now(),
    published_at   timestamptz,
    deleted_at     timestamptz
);
CREATE INDEX presets_browse ON presets (created_at DESC, id DESC) WHERE status = 'published' AND deleted_at IS NULL;
CREATE INDEX presets_author ON presets (author_id, created_at DESC);
CREATE INDEX presets_printer ON presets (printer_id);
CREATE INDEX presets_material ON presets (material_id);
CREATE INDEX presets_slicer ON presets (slicer_id);

-- One row shape for every preset read: the preset with its author and catalog names.
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

-- +goose Down
DROP VIEW preset_view;
DROP TABLE presets;

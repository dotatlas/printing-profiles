-- +goose Up
-- The catalog: the printers, filaments and slicers that presets refer to.

-- A brand can make printers, filaments, or both.
CREATE TABLE brands (
    id   bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name text NOT NULL UNIQUE,
    slug text NOT NULL UNIQUE CHECK (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$')
);

CREATE TABLE printers (
    id       bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    brand_id bigint NOT NULL REFERENCES brands (id),
    model    text NOT NULL,
    slug     text NOT NULL UNIQUE CHECK (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$'),
    UNIQUE (brand_id, model)
);

-- A material is one filament product in one diameter.
CREATE TABLE materials (
    id           bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    brand_id     bigint NOT NULL REFERENCES brands (id),
    product_name text NOT NULL,
    type         text NOT NULL CHECK (type IN ('PLA', 'PETG', 'ABS', 'ASA', 'TPU', 'PA', 'PC')),
    diameter_mm  numeric(3, 2) NOT NULL CHECK (diameter_mm IN (1.75, 2.85)),
    slug         text NOT NULL UNIQUE CHECK (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$'),
    UNIQUE (brand_id, product_name, diameter_mm)
);

CREATE TABLE slicers (
    id   bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name text NOT NULL UNIQUE,
    slug text NOT NULL UNIQUE CHECK (slug ~ '^[a-z0-9]+(-[a-z0-9]+)*$')
);

-- +goose Down
DROP TABLE slicers;
DROP TABLE materials;
DROP TABLE printers;
DROP TABLE brands;

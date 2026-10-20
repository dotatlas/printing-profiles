-- +goose Up
-- A filament product is sold in many colors. The print settings do not change with the color, so a color is not a material.
CREATE TABLE material_colors (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    material_id bigint NOT NULL REFERENCES materials (id),
    name        text NOT NULL CHECK (name <> ''),
    hex         text NOT NULL CHECK (hex ~ '^#[0-9a-f]{6}$'),
    UNIQUE (material_id, name)
);

-- Every seeded filament line is sold in black and white. Contributors add the other colors later.
INSERT INTO material_colors (material_id, name, hex)
SELECT m.id, c.name, c.hex
FROM materials m
CROSS JOIN (VALUES ('Black', '#000000'), ('White', '#ffffff')) AS c (name, hex);

-- +goose Down
DROP TABLE material_colors;

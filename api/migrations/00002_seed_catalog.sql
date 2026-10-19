-- +goose Up
-- A starter catalog of popular printers, filaments and slicers. All filaments are 1.75 mm.
-- A slug is the brand slug plus the model or product name. The script that writes this file checks that each slug is unique.

INSERT INTO brands (name, slug) VALUES
    ('Bambu Lab', 'bambu-lab'),
    ('Prusa Research', 'prusa-research'),
    ('Creality', 'creality'),
    ('Elegoo', 'elegoo'),
    ('Anycubic', 'anycubic'),
    ('Sovol', 'sovol'),
    ('Polymaker', 'polymaker'),
    ('eSUN', 'esun'),
    ('SUNLU', 'sunlu'),
    ('Overture', 'overture'),
    ('Hatchbox', 'hatchbox');

INSERT INTO printers (brand_id, model, slug)
SELECT b.id, v.model, v.slug
FROM (VALUES
    ('bambu-lab', 'X1 Carbon', 'bambu-lab-x1-carbon'),
    ('bambu-lab', 'P1S', 'bambu-lab-p1s'),
    ('bambu-lab', 'P1P', 'bambu-lab-p1p'),
    ('bambu-lab', 'A1', 'bambu-lab-a1'),
    ('bambu-lab', 'A1 mini', 'bambu-lab-a1-mini'),
    ('bambu-lab', 'H2D', 'bambu-lab-h2d'),
    ('prusa-research', 'MK4S', 'prusa-research-mk4s'),
    ('prusa-research', 'MK4', 'prusa-research-mk4'),
    ('prusa-research', 'MINI+', 'prusa-research-mini-plus'),
    ('prusa-research', 'XL', 'prusa-research-xl'),
    ('prusa-research', 'CORE One', 'prusa-research-core-one'),
    ('creality', 'K1', 'creality-k1'),
    ('creality', 'K1C', 'creality-k1c'),
    ('creality', 'K1 Max', 'creality-k1-max'),
    ('creality', 'K2 Plus', 'creality-k2-plus'),
    ('creality', 'Ender-3 V3', 'creality-ender-3-v3'),
    ('creality', 'Ender-3 V3 SE', 'creality-ender-3-v3-se'),
    ('creality', 'Ender-3 V3 KE', 'creality-ender-3-v3-ke'),
    ('elegoo', 'Neptune 4 Pro', 'elegoo-neptune-4-pro'),
    ('elegoo', 'Centauri Carbon', 'elegoo-centauri-carbon'),
    ('anycubic', 'Kobra 3', 'anycubic-kobra-3'),
    ('anycubic', 'Kobra S1', 'anycubic-kobra-s1'),
    ('sovol', 'SV06', 'sovol-sv06'),
    ('sovol', 'SV08', 'sovol-sv08')
) AS v (brand_slug, model, slug)
JOIN brands b ON b.slug = v.brand_slug;

INSERT INTO materials (brand_id, product_name, type, diameter_mm, slug)
SELECT b.id, v.product_name, v.type, 1.75, v.slug
FROM (VALUES
    ('bambu-lab', 'PLA Basic', 'PLA', 'bambu-lab-pla-basic'),
    ('bambu-lab', 'PLA Matte', 'PLA', 'bambu-lab-pla-matte'),
    ('bambu-lab', 'PETG HF', 'PETG', 'bambu-lab-petg-hf'),
    ('bambu-lab', 'ABS', 'ABS', 'bambu-lab-abs'),
    ('bambu-lab', 'ASA', 'ASA', 'bambu-lab-asa'),
    ('bambu-lab', 'TPU 95A HF', 'TPU', 'bambu-lab-tpu-95a-hf'),
    ('prusa-research', 'Prusament PLA', 'PLA', 'prusa-research-prusament-pla'),
    ('prusa-research', 'Prusament PETG', 'PETG', 'prusa-research-prusament-petg'),
    ('prusa-research', 'Prusament ASA', 'ASA', 'prusa-research-prusament-asa'),
    ('polymaker', 'PolyTerra PLA', 'PLA', 'polymaker-polyterra-pla'),
    ('polymaker', 'PolyLite PETG', 'PETG', 'polymaker-polylite-petg'),
    ('polymaker', 'PolyLite ABS', 'ABS', 'polymaker-polylite-abs'),
    ('polymaker', 'PolyLite ASA', 'ASA', 'polymaker-polylite-asa'),
    ('esun', 'PLA+', 'PLA', 'esun-pla-plus'),
    ('esun', 'PETG', 'PETG', 'esun-petg'),
    ('sunlu', 'PLA', 'PLA', 'sunlu-pla'),
    ('sunlu', 'PETG', 'PETG', 'sunlu-petg'),
    ('overture', 'PLA', 'PLA', 'overture-pla'),
    ('overture', 'PETG', 'PETG', 'overture-petg'),
    ('overture', 'TPU', 'TPU', 'overture-tpu'),
    ('hatchbox', 'PLA', 'PLA', 'hatchbox-pla'),
    ('hatchbox', 'PETG', 'PETG', 'hatchbox-petg'),
    ('elegoo', 'PLA', 'PLA', 'elegoo-pla')
) AS v (brand_slug, product_name, type, slug)
JOIN brands b ON b.slug = v.brand_slug;

INSERT INTO slicers (name, slug) VALUES
    ('OrcaSlicer', 'orcaslicer'),
    ('PrusaSlicer', 'prusaslicer'),
    ('Bambu Studio', 'bambu-studio'),
    ('UltiMaker Cura', 'ultimaker-cura'),
    ('SuperSlicer', 'superslicer'),
    ('Creality Print', 'creality-print');

-- +goose Down
-- Remove only the rows that the Up section added.
DELETE FROM slicers WHERE slug IN ('orcaslicer', 'prusaslicer', 'bambu-studio', 'ultimaker-cura', 'superslicer', 'creality-print');
DELETE FROM materials WHERE slug IN ('bambu-lab-pla-basic', 'bambu-lab-pla-matte', 'bambu-lab-petg-hf', 'bambu-lab-abs', 'bambu-lab-asa', 'bambu-lab-tpu-95a-hf', 'prusa-research-prusament-pla', 'prusa-research-prusament-petg', 'prusa-research-prusament-asa', 'polymaker-polyterra-pla', 'polymaker-polylite-petg', 'polymaker-polylite-abs', 'polymaker-polylite-asa', 'esun-pla-plus', 'esun-petg', 'sunlu-pla', 'sunlu-petg', 'overture-pla', 'overture-petg', 'overture-tpu', 'hatchbox-pla', 'hatchbox-petg', 'elegoo-pla');
DELETE FROM printers WHERE slug IN ('bambu-lab-x1-carbon', 'bambu-lab-p1s', 'bambu-lab-p1p', 'bambu-lab-a1', 'bambu-lab-a1-mini', 'bambu-lab-h2d', 'prusa-research-mk4s', 'prusa-research-mk4', 'prusa-research-mini-plus', 'prusa-research-xl', 'prusa-research-core-one', 'creality-k1', 'creality-k1c', 'creality-k1-max', 'creality-k2-plus', 'creality-ender-3-v3', 'creality-ender-3-v3-se', 'creality-ender-3-v3-ke', 'elegoo-neptune-4-pro', 'elegoo-centauri-carbon', 'anycubic-kobra-3', 'anycubic-kobra-s1', 'sovol-sv06', 'sovol-sv08');
DELETE FROM brands WHERE slug IN ('bambu-lab', 'prusa-research', 'creality', 'elegoo', 'anycubic', 'sovol', 'polymaker', 'esun', 'sunlu', 'overture', 'hatchbox');

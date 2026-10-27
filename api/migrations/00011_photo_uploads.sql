-- +goose Up
-- Uploads take photos (up to 10 MB), and keep the size and the thumbnail of a checked photo.

ALTER TABLE uploads DROP CONSTRAINT uploads_kind_check;
ALTER TABLE uploads ADD CONSTRAINT uploads_kind_check CHECK (kind IN ('preset_file', 'photo'));
ALTER TABLE uploads DROP CONSTRAINT uploads_size_check;
ALTER TABLE uploads ADD CONSTRAINT uploads_size_check CHECK (size BETWEEN 1 AND 10485760);
ALTER TABLE uploads ADD COLUMN thumb_key text NOT NULL DEFAULT '';
ALTER TABLE uploads ADD COLUMN width integer NOT NULL DEFAULT 0;
ALTER TABLE uploads ADD COLUMN height integer NOT NULL DEFAULT 0;

-- +goose Down
DELETE FROM uploads WHERE kind = 'photo';
ALTER TABLE uploads DROP COLUMN height;
ALTER TABLE uploads DROP COLUMN width;
ALTER TABLE uploads DROP COLUMN thumb_key;
ALTER TABLE uploads DROP CONSTRAINT uploads_size_check;
ALTER TABLE uploads ADD CONSTRAINT uploads_size_check CHECK (size BETWEEN 1 AND 1048576);
ALTER TABLE uploads DROP CONSTRAINT uploads_kind_check;
ALTER TABLE uploads ADD CONSTRAINT uploads_kind_check CHECK (kind IN ('preset_file'));

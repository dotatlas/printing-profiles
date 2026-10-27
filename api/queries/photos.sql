-- Result photos. Only the author of a preset adds and removes its photos; the author_id conditions make another
-- member's preset look missing.

-- name: MarkPhotoReady :execrows
-- Only a pending upload changes, so two "complete" calls at the same time cannot both write.
UPDATE uploads SET status = 'ready', sha256 = sqlc.arg(sha256), file_key = sqlc.arg(file_key),
    thumb_key = sqlc.arg(thumb_key), width = sqlc.arg(width), height = sqlc.arg(height), completed_at = now()
WHERE id = sqlc.arg(id) AND status = 'pending';

-- name: AuthorPreset :one
-- The author's own preset, a draft or published.
SELECT id FROM presets
WHERE public_id = sqlc.arg(public_id) AND author_id = sqlc.arg(author_id) AND deleted_at IS NULL;

-- name: ReadyPhoto :one
-- A preset can take only its author's own checked photo.
SELECT id FROM uploads
WHERE public_id = sqlc.arg(public_id) AND owner_id = sqlc.arg(owner_id) AND status = 'ready' AND kind = 'photo';

-- name: CountPhotos :one
SELECT count(*)::int FROM photos WHERE preset_id = sqlc.arg(preset_id);

-- name: AddPhoto :exec
INSERT INTO photos (public_id, preset_id, upload_id, alt)
VALUES (sqlc.arg(public_id), sqlc.arg(preset_id), sqlc.arg(upload_id), sqlc.arg(alt));

-- name: ListPhotos :many
-- Oldest first, so the first photo that the author added leads.
SELECT ph.public_id, ph.alt, u.file_key, u.thumb_key, u.width, u.height
FROM photos ph
JOIN uploads u ON u.id = ph.upload_id
WHERE ph.preset_id = sqlc.arg(preset_id)
ORDER BY ph.created_at, ph.id;

-- name: DeletePhoto :execrows
DELETE FROM photos ph USING presets p
WHERE ph.preset_id = p.id AND ph.public_id = sqlc.arg(public_id) AND p.author_id = sqlc.arg(author_id)
  AND p.deleted_at IS NULL;

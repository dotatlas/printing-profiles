-- Presets. Every read uses preset_view, so the handlers map one row type. Browse lists only published presets,
-- newest first, with keyset pagination on (created_at, id).

-- name: PrinterID :one
SELECT id FROM printers WHERE slug = sqlc.arg(slug);

-- name: MaterialID :one
SELECT id FROM materials WHERE slug = sqlc.arg(slug);

-- name: SlicerID :one
SELECT id FROM slicers WHERE slug = sqlc.arg(slug);

-- name: CreatePreset :exec
INSERT INTO presets (public_id, author_id, title, slug, description, printer_id, material_id, nozzle_mm, slicer_id,
                     slicer_version, license, status, published_at, file_upload_id)
VALUES (sqlc.arg(public_id), sqlc.arg(author_id), sqlc.arg(title), sqlc.arg(slug), sqlc.arg(description),
        sqlc.arg(printer_id), sqlc.arg(material_id), sqlc.arg(nozzle_mm)::float8::numeric(3, 2), sqlc.arg(slicer_id),
        sqlc.arg(slicer_version), sqlc.arg(license), sqlc.arg(status),
        CASE WHEN sqlc.arg(status)::text = 'published' THEN now() END, sqlc.narg(file_upload_id));

-- name: GetPreset :one
-- Deleted and hidden presets look like missing ones. The handler shows a draft only to its author.
SELECT * FROM preset_view
WHERE public_id = sqlc.arg(public_id) AND deleted_at IS NULL AND status <> 'hidden';

-- name: ListPresets :many
SELECT * FROM preset_view
WHERE status = 'published' AND deleted_at IS NULL
  AND (sqlc.narg(printer)::text IS NULL OR printer_slug = sqlc.narg(printer))
  AND (sqlc.narg(material)::text IS NULL OR material_slug = sqlc.narg(material))
  AND (sqlc.narg(type)::text IS NULL OR material_type = sqlc.narg(type))
  AND (sqlc.narg(brand)::text IS NULL OR material_brand_slug = sqlc.narg(brand))
  AND (sqlc.narg(nozzle)::float8 IS NULL OR nozzle_mm = sqlc.narg(nozzle))
  AND (sqlc.narg(slicer)::text IS NULL OR slicer_slug = sqlc.narg(slicer))
  AND (sqlc.narg(author)::text IS NULL OR author_handle = sqlc.narg(author))
  AND (NOT sqlc.arg(verified_only)::bool OR photo_count > 0)
  AND (sqlc.narg(after_created)::timestamptz IS NULL
       OR (created_at, id) < (sqlc.narg(after_created)::timestamptz, sqlc.narg(after_id)::bigint))
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(max_rows)::int;

-- name: ListMyPresets :many
-- shortcut: the newest 100 only; page it when someone has more than 100 presets.
SELECT * FROM preset_view
WHERE author_id = sqlc.arg(author_id) AND deleted_at IS NULL
ORDER BY created_at DESC, id DESC
LIMIT 100;

-- name: UpdatePreset :one
-- Each field changes only when it is given. The author_id condition makes another user's preset look missing.
UPDATE presets SET
    title          = coalesce(sqlc.narg(title), title),
    slug           = coalesce(sqlc.narg(slug), slug),
    description    = coalesce(sqlc.narg(description), description),
    printer_id     = coalesce(sqlc.narg(printer_id), printer_id),
    material_id    = coalesce(sqlc.narg(material_id), material_id),
    nozzle_mm      = coalesce(sqlc.narg(nozzle_mm)::float8::numeric(3, 2), nozzle_mm),
    slicer_id      = coalesce(sqlc.narg(slicer_id), slicer_id),
    slicer_version = coalesce(sqlc.narg(slicer_version), slicer_version),
    license        = coalesce(sqlc.narg(license), license),
    file_upload_id = coalesce(sqlc.narg(file_upload_id), file_upload_id),
    status         = coalesce(sqlc.narg(status), status),
    published_at   = CASE WHEN coalesce(sqlc.narg(status), status) = 'published' THEN coalesce(published_at, now()) ELSE published_at END,
    updated_at     = now()
WHERE public_id = sqlc.arg(public_id) AND author_id = sqlc.arg(author_id) AND deleted_at IS NULL AND status <> 'hidden'
RETURNING public_id;

-- name: DeletePreset :execrows
UPDATE presets SET deleted_at = now()
WHERE public_id = sqlc.arg(public_id) AND author_id = sqlc.arg(author_id) AND deleted_at IS NULL;

-- name: ReadyUpload :one
-- A preset can take only its author's own checked preset file.
SELECT id, slicer FROM uploads
WHERE public_id = sqlc.arg(public_id) AND owner_id = sqlc.arg(owner_id) AND status = 'ready' AND kind = 'preset_file';

-- name: CountDownload :exec
-- shortcut: counts every request, so a script can raise the count; count once per IP and day with the moderation work.
UPDATE presets SET download_count = download_count + 1 WHERE id = sqlc.arg(id);

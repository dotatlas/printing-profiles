-- Uploads. A member sees only their own: the owner_id condition makes another member's upload look missing.

-- name: CreateUpload :exec
INSERT INTO uploads (public_id, owner_id, kind, size)
VALUES (sqlc.arg(public_id), sqlc.arg(owner_id), sqlc.arg(kind), sqlc.arg(size));

-- name: GetUpload :one
SELECT * FROM uploads WHERE public_id = sqlc.arg(public_id) AND owner_id = sqlc.arg(owner_id);

-- name: MarkUploadReady :execrows
-- Only a pending upload changes, so two "complete" calls at the same time cannot both write.
UPDATE uploads SET status = 'ready', sha256 = sqlc.arg(sha256), file_key = sqlc.arg(file_key), slicer = sqlc.arg(slicer),
    file_kind = sqlc.arg(file_kind), slicer_version = sqlc.arg(slicer_version), settings = sqlc.arg(settings),
    removed = sqlc.arg(removed), completed_at = now()
WHERE id = sqlc.arg(id) AND status = 'pending';

-- name: MarkUploadRejected :exec
UPDATE uploads SET status = 'rejected', reason = sqlc.arg(reason), completed_at = now()
WHERE id = sqlc.arg(id) AND status = 'pending';

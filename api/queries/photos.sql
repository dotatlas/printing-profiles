-- Result photos.

-- name: MarkPhotoReady :execrows
-- Only a pending upload changes, so two "complete" calls at the same time cannot both write.
UPDATE uploads SET status = 'ready', sha256 = sqlc.arg(sha256), file_key = sqlc.arg(file_key),
    thumb_key = sqlc.arg(thumb_key), width = sqlc.arg(width), height = sqlc.arg(height), completed_at = now()
WHERE id = sqlc.arg(id) AND status = 'pending';

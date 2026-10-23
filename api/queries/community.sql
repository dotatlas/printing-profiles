-- Reviews and comments. The owner condition on every change makes another member's text look missing.

-- name: PublishedPreset :one
-- Reviews go only on a published preset; the author is needed to refuse a review of one's own preset.
SELECT id, author_id FROM presets
WHERE public_id = sqlc.arg(public_id) AND status = 'published' AND deleted_at IS NULL;

-- name: UpsertReview :one
-- One review per member per preset: a second one replaces the first. created is false for a replacement.
INSERT INTO reviews (preset_id, author_id, stars, body)
VALUES (sqlc.arg(preset_id), sqlc.arg(author_id), sqlc.arg(stars), sqlc.arg(body))
ON CONFLICT (preset_id, author_id) DO UPDATE SET stars = EXCLUDED.stars, body = EXCLUDED.body, updated_at = now()
RETURNING (xmax = 0)::boolean AS created;

-- name: DeleteMyReview :execrows
DELETE FROM reviews WHERE preset_id = sqlc.arg(preset_id) AND author_id = sqlc.arg(author_id);

-- name: ListReviews :many
SELECT r.id, r.stars, r.body, r.created_at, r.updated_at,
       coalesce(u.handle, '')::text AS author_handle, u.display_name AS author_display_name
FROM reviews r
JOIN users u ON u.id = r.author_id
WHERE r.preset_id = sqlc.arg(preset_id)
  AND (sqlc.narg(after_created)::timestamptz IS NULL
       OR (r.created_at, r.id) < (sqlc.narg(after_created)::timestamptz, sqlc.narg(after_id)::bigint))
ORDER BY r.created_at DESC, r.id DESC
LIMIT sqlc.arg(max_rows)::int;

-- name: MyReview :one
SELECT stars, body, created_at, updated_at FROM reviews
WHERE preset_id = sqlc.arg(preset_id) AND author_id = sqlc.arg(author_id);

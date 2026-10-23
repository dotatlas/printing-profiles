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

-- name: CommentParent :one
-- A reply goes only under a top-level comment of the same preset that is not deleted: one level of replies.
SELECT id FROM comments
WHERE public_id = sqlc.arg(public_id) AND preset_id = sqlc.arg(preset_id) AND parent_id IS NULL AND deleted_at IS NULL;

-- name: CreateComment :exec
INSERT INTO comments (public_id, preset_id, author_id, parent_id, body)
VALUES (sqlc.arg(public_id), sqlc.arg(preset_id), sqlc.arg(author_id), sqlc.narg(parent_id), sqlc.arg(body));

-- name: UpdateComment :execrows
UPDATE comments SET body = sqlc.arg(body), updated_at = now()
WHERE public_id = sqlc.arg(public_id) AND author_id = sqlc.arg(author_id) AND deleted_at IS NULL;

-- name: DeleteComment :execrows
-- The text goes at once; the row stays for the replies under it.
UPDATE comments SET body = '', deleted_at = now()
WHERE public_id = sqlc.arg(public_id) AND author_id = sqlc.arg(author_id) AND deleted_at IS NULL;

-- name: GetComment :one
SELECT c.public_id, c.body, c.created_at, c.updated_at, c.deleted_at,
       coalesce(u.handle, '')::text AS author_handle, u.display_name AS author_display_name
FROM comments c
JOIN users u ON u.id = c.author_id
WHERE c.public_id = sqlc.arg(public_id);

-- name: ListTopComments :many
-- Oldest first. A deleted comment shows only when a reply under it is still there.
SELECT c.id, c.public_id, c.body, c.created_at, c.updated_at, c.deleted_at,
       coalesce(u.handle, '')::text AS author_handle, u.display_name AS author_display_name
FROM comments c
JOIN users u ON u.id = c.author_id
WHERE c.preset_id = sqlc.arg(preset_id) AND c.parent_id IS NULL
  AND (c.deleted_at IS NULL OR EXISTS (SELECT 1 FROM comments r WHERE r.parent_id = c.id AND r.deleted_at IS NULL))
  AND (sqlc.narg(after_created)::timestamptz IS NULL
       OR (c.created_at, c.id) > (sqlc.narg(after_created)::timestamptz, sqlc.narg(after_id)::bigint))
ORDER BY c.created_at, c.id
LIMIT sqlc.arg(max_rows)::int;

-- name: ListReplies :many
-- shortcut: every reply of the page's comments comes at once; page the replies when threads grow past a few dozen.
SELECT c.parent_id, c.public_id, c.body, c.created_at, c.updated_at,
       coalesce(u.handle, '')::text AS author_handle, u.display_name AS author_display_name
FROM comments c
JOIN users u ON u.id = c.author_id
WHERE c.parent_id = ANY(sqlc.arg(parent_ids)::bigint[]) AND c.deleted_at IS NULL
ORDER BY c.created_at, c.id;

-- name: AddBookmark :exec
-- Adding a bookmark twice changes nothing.
INSERT INTO bookmarks (user_id, preset_id) VALUES (sqlc.arg(user_id), sqlc.arg(preset_id))
ON CONFLICT DO NOTHING;

-- name: RemoveBookmark :exec
DELETE FROM bookmarks WHERE user_id = sqlc.arg(user_id)
  AND preset_id = (SELECT id FROM presets WHERE public_id = sqlc.arg(public_id));

-- name: IsBookmarked :one
SELECT EXISTS (
    SELECT 1 FROM bookmarks b JOIN presets p ON p.id = b.preset_id
    WHERE b.user_id = sqlc.arg(user_id) AND p.public_id = sqlc.arg(public_id)
)::boolean;

-- name: ListBookmarks :many
-- shortcut: the newest 100 only; page it when members save more than 100 presets.
-- A preset that is no longer published drops out of the list, and comes back if it is published again.
SELECT v.* FROM bookmarks b
JOIN preset_view v ON v.id = b.preset_id
WHERE b.user_id = sqlc.arg(user_id) AND v.status = 'published' AND v.deleted_at IS NULL
ORDER BY b.created_at DESC
LIMIT 100;

-- name: GetProfile :one
-- A public profile: only a user who chose a handle has one.
SELECT u.handle::text AS handle, u.display_name, u.created_at,
       (SELECT count(*) FROM presets p WHERE p.author_id = u.id AND p.status = 'published' AND p.deleted_at IS NULL)::bigint AS preset_count,
       (SELECT count(*) FROM reviews r WHERE r.author_id = u.id)::bigint AS review_count
FROM users u
WHERE lower(u.handle) = lower(sqlc.arg(handle));

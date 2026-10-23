-- +goose Up
-- Bookmarks: the presets that a member saved for later.

CREATE TABLE bookmarks (
    user_id    bigint NOT NULL REFERENCES users (id),
    preset_id  bigint NOT NULL REFERENCES presets (id),
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, preset_id)
);
CREATE INDEX bookmarks_user ON bookmarks (user_id, created_at DESC);

-- +goose Down
DROP TABLE bookmarks;

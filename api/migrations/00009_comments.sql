-- +goose Up
-- Comments on a preset, with one level of replies. A deleted comment loses its text but keeps its row, so the
-- replies under it keep their place.

CREATE TABLE comments (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id  text NOT NULL UNIQUE CHECK (public_id ~ '^[a-z2-9]{12}$'),
    preset_id  bigint NOT NULL REFERENCES presets (id),
    author_id  bigint NOT NULL REFERENCES users (id),
    parent_id  bigint REFERENCES comments (id),
    body       text NOT NULL CHECK (length(body) <= 2000),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);
CREATE INDEX comments_preset ON comments (preset_id, created_at, id) WHERE parent_id IS NULL;
CREATE INDEX comments_parent ON comments (parent_id, created_at, id);
CREATE INDEX comments_author ON comments (author_id);

-- +goose Down
DROP TABLE comments;

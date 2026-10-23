-- +goose Up
-- Uploads: a file that a member sends straight to the private bucket. "complete" reads it, checks it, and stores a
-- clean copy in the public files bucket. The row keeps what the check found.

CREATE TABLE uploads (
    id             bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    public_id      text NOT NULL UNIQUE CHECK (public_id ~ '^[a-z2-9]{16}$'),
    owner_id       bigint NOT NULL REFERENCES users (id),
    kind           text NOT NULL CHECK (kind IN ('preset_file')),
    size           integer NOT NULL CHECK (size BETWEEN 1 AND 1048576),
    status         text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'ready', 'rejected')),
    reason         text NOT NULL DEFAULT '',          -- why the check refused the file
    sha256         text NOT NULL DEFAULT '',          -- of the clean copy
    file_key       text NOT NULL DEFAULT '',          -- the key of the clean copy in the files bucket
    slicer         text NOT NULL DEFAULT '',
    file_kind      text NOT NULL DEFAULT '',          -- print, filament, printer or config
    slicer_version text NOT NULL DEFAULT '',
    settings       jsonb NOT NULL DEFAULT '[]',       -- the key settings: [{id, label, value, unit}]
    removed        text[] NOT NULL DEFAULT '{}',      -- the connection secrets that the check removed
    created_at     timestamptz NOT NULL DEFAULT now(),
    completed_at   timestamptz
);
CREATE INDEX uploads_owner ON uploads (owner_id, created_at DESC);

-- +goose Down
DROP TABLE uploads;

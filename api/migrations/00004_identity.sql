-- +goose Up
-- Sign-in: users, the accounts they sign in with, their sessions, and the short-lived rows of each sign-in flow.

-- A user gets a handle at onboarding, so it is empty until then.
CREATE TABLE users (
    id                bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    handle            text CHECK (handle ~ '^[a-z0-9]+(-[a-z0-9]+)*$' AND length(handle) BETWEEN 3 AND 30),
    display_name      text NOT NULL DEFAULT '' CHECK (length(display_name) <= 60),
    terms_accepted_at timestamptz,
    created_at        timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX users_handle_key ON users (lower(handle));

-- One row for each way that a user signs in. For email, provider_user_id is the normalized address.
CREATE TABLE identities (
    id               bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id          bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    provider         text NOT NULL CHECK (provider IN ('github', 'discord', 'email', 'fake')),
    provider_user_id text NOT NULL CHECK (provider_user_id <> ''),
    email            text NOT NULL DEFAULT '',
    created_at       timestamptz NOT NULL DEFAULT now(),
    UNIQUE (provider, provider_user_id)
);
CREATE INDEX identities_user_id ON identities (user_id);

-- The cookie holds the token. The database holds only its SHA-256.
CREATE TABLE sessions (
    id           bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    token_hash   bytea NOT NULL UNIQUE,
    user_id      bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    user_agent   text NOT NULL DEFAULT '',
    created_at   timestamptz NOT NULL DEFAULT now(),
    last_seen_at timestamptz NOT NULL DEFAULT now(),
    expires_at   timestamptz NOT NULL
);
CREATE INDEX sessions_user_id ON sessions (user_id);

-- One row for each OAuth sign-in that started. The callback deletes it, so each state works one time.
CREATE TABLE oauth_flows (
    state_hash   bytea PRIMARY KEY,
    provider     text NOT NULL,
    verifier     text NOT NULL,
    return_path  text NOT NULL,
    link_user_id bigint REFERENCES users (id) ON DELETE CASCADE,
    expires_at   timestamptz NOT NULL
);

-- One row for each sign-in link sent by email. used_at makes each link work one time.
CREATE TABLE email_tokens (
    token_hash  bytea PRIMARY KEY,
    email       text NOT NULL,
    return_path text NOT NULL,
    expires_at  timestamptz NOT NULL,
    used_at     timestamptz
);

-- Request counts in fixed time windows. The key is an HMAC of the action and the client, never the plain IP or address.
CREATE TABLE rate_limits (
    key          bytea NOT NULL,
    window_start timestamptz NOT NULL,
    count        integer NOT NULL,
    PRIMARY KEY (key, window_start)
);

-- +goose Down
DROP TABLE rate_limits;
DROP TABLE email_tokens;
DROP TABLE oauth_flows;
DROP TABLE sessions;
DROP TABLE identities;
DROP TABLE users;

-- Users, identities and sessions. The session rules live here: 30 days from sign-in, 7 days without use,
-- and last_seen_at moves at most once an hour, so reads do not write on every request.

-- name: FindUserByIdentity :one
SELECT user_id FROM identities
WHERE provider = sqlc.arg(provider) AND provider_user_id = sqlc.arg(provider_user_id);

-- name: CreateUserWithIdentity :one
-- One statement, so a failed identity insert (a race on the unique key) also removes the new user.
WITH new_user AS (
    INSERT INTO users DEFAULT VALUES RETURNING id
)
INSERT INTO identities (user_id, provider, provider_user_id, email)
SELECT id, sqlc.arg(provider), sqlc.arg(provider_user_id), sqlc.arg(email) FROM new_user
RETURNING user_id;

-- name: CreateSession :exec
INSERT INTO sessions (token_hash, user_id, user_agent, expires_at)
VALUES (sqlc.arg(token_hash), sqlc.arg(user_id), sqlc.arg(user_agent), now() + interval '30 days');

-- name: DeleteExpiredSessions :exec
DELETE FROM sessions
WHERE user_id = sqlc.arg(user_id) AND (expires_at <= now() OR last_seen_at <= now() - interval '7 days');

-- name: GetSession :one
SELECT s.id, s.user_id, s.last_seen_at, coalesce(u.handle, '')::text AS handle, u.display_name
FROM sessions s
JOIN users u ON u.id = s.user_id
WHERE s.token_hash = sqlc.arg(token_hash)
  AND s.expires_at > now()
  AND s.last_seen_at > now() - interval '7 days';

-- name: TouchSession :exec
UPDATE sessions SET last_seen_at = now()
WHERE id = sqlc.arg(id) AND last_seen_at < now() - interval '1 hour';

-- name: ListSessions :many
SELECT id, user_agent, created_at, last_seen_at FROM sessions
WHERE user_id = sqlc.arg(user_id) AND expires_at > now() AND last_seen_at > now() - interval '7 days'
ORDER BY last_seen_at DESC, id;

-- name: DeleteSession :execrows
-- The user_id condition makes another user's session look like a missing one.
DELETE FROM sessions WHERE id = sqlc.arg(id) AND user_id = sqlc.arg(user_id);

-- name: DeleteUserSessions :exec
DELETE FROM sessions WHERE user_id = sqlc.arg(user_id);

-- name: CreateOAuthFlow :exec
INSERT INTO oauth_flows (state_hash, provider, verifier, return_path, link_user_id, expires_at)
VALUES (sqlc.arg(state_hash), sqlc.arg(provider), sqlc.arg(verifier), sqlc.arg(return_path),
        sqlc.narg(link_user_id), now() + interval '10 minutes');

-- name: TakeOAuthFlow :one
-- DELETE ... RETURNING makes each state work one time, even when two callbacks arrive together.
DELETE FROM oauth_flows
WHERE state_hash = sqlc.arg(state_hash) AND provider = sqlc.arg(provider) AND expires_at > now()
RETURNING verifier, return_path, link_user_id;

-- name: DeleteExpiredOAuthFlows :exec
DELETE FROM oauth_flows WHERE expires_at <= now();

-- name: AddIdentity :exec
INSERT INTO identities (user_id, provider, provider_user_id, email)
VALUES (sqlc.arg(user_id), sqlc.arg(provider), sqlc.arg(provider_user_id), sqlc.arg(email));

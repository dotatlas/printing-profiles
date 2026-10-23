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

-- name: DeleteOtherSessions :execrows
DELETE FROM sessions WHERE user_id = sqlc.arg(user_id) AND id <> sqlc.arg(keep_id);

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

-- name: HitRateLimit :one
-- Count one more request in this window and return the new count.
INSERT INTO rate_limits (key, window_start, count) VALUES (sqlc.arg(key), sqlc.arg(window_start), 1)
ON CONFLICT (key, window_start) DO UPDATE SET count = rate_limits.count + 1
RETURNING count;

-- name: DeleteOldRateLimits :exec
DELETE FROM rate_limits WHERE window_start < now() - interval '1 day';

-- name: CreateEmailToken :exec
INSERT INTO email_tokens (token_hash, email, return_path, expires_at)
VALUES (sqlc.arg(token_hash), sqlc.arg(email), sqlc.arg(return_path), now() + interval '15 minutes');

-- name: UseEmailToken :one
-- used_at IS NULL in the WHERE makes each link work one time, even when two requests arrive together.
UPDATE email_tokens SET used_at = now()
WHERE token_hash = sqlc.arg(token_hash) AND used_at IS NULL AND expires_at > now()
RETURNING email, return_path;

-- name: DeleteOldEmailTokens :exec
DELETE FROM email_tokens WHERE expires_at <= now() - interval '1 day';

-- name: FinishOnboarding :execrows
-- Only a user with no handle can onboard, so a handle never changes here.
UPDATE users SET handle = sqlc.arg(handle), terms_accepted_at = now()
WHERE id = sqlc.arg(id) AND handle IS NULL;

-- name: SetDisplayName :exec
UPDATE users SET display_name = sqlc.arg(display_name) WHERE id = sqlc.arg(id);

-- name: ListIdentities :many
SELECT id, provider, CASE WHEN provider = 'email' THEN provider_user_id ELSE '' END::text AS email, created_at
FROM identities WHERE user_id = sqlc.arg(user_id)
ORDER BY created_at, id;

-- name: DeleteIdentity :execrows
-- shortcut: two parallel deletes can remove the last two identities; it needs two clicks at the same moment, so use a locked transaction if it ever happens.
DELETE FROM identities i
WHERE i.id = sqlc.arg(id) AND i.user_id = sqlc.arg(user_id)
  AND EXISTS (SELECT 1 FROM identities o WHERE o.user_id = sqlc.arg(user_id) AND o.id <> sqlc.arg(id));

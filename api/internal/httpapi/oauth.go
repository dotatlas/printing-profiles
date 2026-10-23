package httpapi

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/endpoints"

	"github.com/dotatlas/printing-profiles/api/internal/config"
	"github.com/dotatlas/printing-profiles/api/internal/db"
)

// oauthCookie holds the state of one OAuth sign-in in the browser, so a callback from another browser fails.
const oauthCookie = "__Host-oauth"

// providerTimeout is the longest time that one call to a provider can take.
const providerTimeout = 10 * time.Second

// oauthProvider is one OAuth provider: its app settings and the URL that tells who the user is.
type oauthProvider struct {
	conf    oauth2.Config
	userURL string // a GET with the access token returns JSON with the user's "id"
}

// providersFrom makes the providers that cfg has an OAuth app for. Handler calls it when s.providers is not set.
func providersFrom(cfg config.Config) map[string]*oauthProvider {
	ps := map[string]*oauthProvider{}
	add := func(name string, c config.OAuthClient, ep oauth2.Endpoint, userURL string, scopes ...string) {
		if c.ID == "" {
			return
		}
		ps[name] = &oauthProvider{userURL: userURL, conf: oauth2.Config{
			ClientID: c.ID, ClientSecret: c.Secret, Endpoint: ep, Scopes: scopes,
			RedirectURL: cfg.APIOrigin + "/api/v1/auth/" + name + "/callback",
		}}
	}
	// Ask for the smallest scope: the API keeps only the account ID, not the email or the profile.
	add("github", cfg.GitHub, endpoints.GitHub, "https://api.github.com/user")
	add("discord", cfg.Discord, endpoints.Discord, "https://discord.com/api/users/@me", "identify")
	return ps
}

// handleOAuthStart sends the browser to a provider with a new state and a PKCE challenge.
// The router calls it for GET /api/v1/auth/{provider}/start?return=<path>[&link=1].
func (s *Server) handleOAuthStart(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("provider")
	p, ok := s.providers[name]
	if !ok {
		writeProblem(w, http.StatusNotFound, "unknown sign-in provider")
		return
	}
	if !s.allow(w, r, "oauth-start", s.clientIP(r), 30, time.Hour) {
		return
	}
	var linkUser pgtype.Int8
	if r.URL.Query().Get("link") == "1" {
		u, ok := requireUser(w, r) // Only a signed-in user can add a provider to an account.
		if !ok {
			return
		}
		linkUser = pgtype.Int8{Int64: u.UserID, Valid: true}
	}
	ctx := r.Context()
	if err := s.DB.DeleteExpiredOAuthFlows(ctx); err != nil {
		s.fail(w, r, err)
		return
	}
	state, stateHash := newToken()
	verifier := oauth2.GenerateVerifier()
	err := s.DB.CreateOAuthFlow(ctx, db.CreateOAuthFlowParams{
		StateHash: stateHash, Provider: name, Verifier: verifier,
		ReturnPath: safeReturnPath(r.URL.Query().Get("return")), LinkUserID: linkUser,
	})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	// Lax, because the provider sends the browser back with a top-level GET, which carries Lax cookies.
	http.SetCookie(w, &http.Cookie{Name: oauthCookie, Value: state, Path: "/", MaxAge: 600, Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode})
	http.Redirect(w, r, p.conf.AuthCodeURL(state, oauth2.S256ChallengeOption(verifier)), http.StatusFound)
}

// handleOAuthCallback finishes a sign-in: it checks the state, exchanges the code, and signs in or links the account.
// The router calls it for GET /api/v1/auth/{provider}/callback?code=...&state=....
func (s *Server) handleOAuthCallback(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("provider")
	p, ok := s.providers[name]
	if !ok {
		writeProblem(w, http.StatusNotFound, "unknown sign-in provider")
		return
	}
	http.SetCookie(w, &http.Cookie{Name: oauthCookie, Path: "/", MaxAge: -1, Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode})

	q := r.URL.Query()
	state := q.Get("state")
	c, err := r.Cookie(oauthCookie)
	if err != nil || state == "" || subtle.ConstantTimeCompare([]byte(c.Value), []byte(state)) != 1 {
		// A forged callback, or one that arrived in another browser.
		writeProblem(w, http.StatusBadRequest, "this sign-in did not start in this browser; start it again")
		return
	}
	ctx := r.Context()
	flow, err := s.DB.TakeOAuthFlow(ctx, db.TakeOAuthFlowParams{StateHash: hashToken(state), Provider: name})
	if errors.Is(err, pgx.ErrNoRows) {
		writeProblem(w, http.StatusBadRequest, "this sign-in expired or was already used; start it again")
		return
	}
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if q.Get("error") != "" || q.Get("code") == "" {
		s.redirectToSite(w, r, "/signin?error=cancelled") // The user said no at the provider.
		return
	}

	accountID, err := p.accountID(ctx, q.Get("code"), flow.Verifier)
	if err != nil {
		s.Log.Error("oauth exchange failed", "request_id", requestIDFrom(ctx), "provider", name, "err", err)
		s.redirectToSite(w, r, "/signin?error=provider")
		return
	}

	if flow.LinkUserID.Valid {
		s.finishLink(w, r, name, accountID, flow.LinkUserID.Int64)
		return
	}
	userID, err := s.signIn(ctx, name, accountID, "")
	if err == nil {
		err = s.startSession(w, r, userID)
	}
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.redirectToSite(w, r, flow.ReturnPath)
}

// finishLink adds a provider account to the user who started the link, if that user is still signed in here.
// handleOAuthCallback calls it. An account that belongs to another user is never moved.
func (s *Server) finishLink(w http.ResponseWriter, r *http.Request, provider, accountID string, userID int64) {
	ctx := r.Context()
	u, ok := userFrom(ctx)
	if !ok || u.UserID != userID {
		s.redirectToSite(w, r, "/signin?error=link-session")
		return
	}
	owner, err := s.DB.FindUserByIdentity(ctx, db.FindUserByIdentityParams{Provider: provider, ProviderUserID: accountID})
	switch {
	case err == nil && owner == userID:
		s.redirectToSite(w, r, "/settings") // It is already linked to this user.
	case err == nil:
		s.redirectToSite(w, r, "/settings?error=linked-elsewhere")
	case errors.Is(err, pgx.ErrNoRows):
		if err := s.DB.AddIdentity(ctx, db.AddIdentityParams{UserID: userID, Provider: provider, ProviderUserID: accountID}); err != nil {
			s.fail(w, r, err)
			return
		}
		s.redirectToSite(w, r, "/settings")
	default:
		s.fail(w, r, err)
	}
}

// accountID exchanges the code (with the PKCE verifier) for a token, and asks the provider for the account ID.
// handleOAuthCallback calls it. Only the ID is kept, so nothing else is read.
func (p *oauthProvider) accountID(ctx context.Context, code, verifier string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, providerTimeout)
	defer cancel()
	client := &http.Client{Timeout: providerTimeout}
	ctx = context.WithValue(ctx, oauth2.HTTPClient, client)

	tok, err := p.conf.Exchange(ctx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		return "", fmt.Errorf("exchange: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.userURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	res, err := p.conf.Client(ctx, tok).Do(req)
	if err != nil {
		return "", fmt.Errorf("user: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("user: status %d", res.StatusCode)
	}
	// GitHub sends the ID as a number and Discord as a string, so read it raw.
	var body struct {
		ID json.RawMessage `json:"id"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&body); err != nil {
		return "", fmt.Errorf("user: %w", err)
	}
	return parseAccountID(body.ID)
}

// parseAccountID reads a provider's account ID, which is a JSON number or a string of digits. accountID calls it.
func parseAccountID(raw json.RawMessage) (string, error) {
	var id string
	if err := json.Unmarshal(raw, &id); err != nil {
		var n json.Number
		if err := json.Unmarshal(raw, &n); err != nil {
			return "", errors.New("user: no account id")
		}
		id = n.String()
	}
	if _, err := strconv.ParseUint(id, 10, 64); err != nil {
		return "", fmt.Errorf("user: account id %q is not a number", id)
	}
	return id, nil
}

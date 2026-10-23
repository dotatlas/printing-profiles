package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/oauth2"

	"github.com/dotatlas/printing-profiles/api/internal/db"
)

// fakeOAuth is a provider for the tests: a token endpoint that checks PKCE, and a user endpoint.
type fakeOAuth struct {
	srv       *httptest.Server
	accountID string // the JSON value of "id" that the user endpoint sends

	mu        sync.Mutex
	challenge string // the last code_challenge from the authorize URL; the test sets it
}

// newFakeOAuth starts a fake provider and returns it. The server stops when the test ends.
func newFakeOAuth(t *testing.T, accountID string) *fakeOAuth {
	f := &fakeOAuth{accountID: accountID}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		sum := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
		f.mu.Lock()
		ok := r.Form.Get("code") == "good-code" && base64.RawURLEncoding.EncodeToString(sum[:]) == f.challenge
		f.mu.Unlock()
		if !ok {
			http.Error(w, `{"error":"invalid_grant"}`, http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"at-1","token_type":"bearer"}`))
	})
	mux.HandleFunc("GET /user", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer at-1" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"id":` + f.accountID + `,"login":"someone"}`))
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

// provider returns the oauthProvider settings that point at this fake.
func (f *fakeOAuth) provider() *oauthProvider {
	return &oauthProvider{userURL: f.srv.URL + "/user", conf: oauth2.Config{
		ClientID: "id", ClientSecret: "secret", RedirectURL: "http://localhost:8080/api/v1/auth/github/callback",
		Endpoint: oauth2.Endpoint{AuthURL: f.srv.URL + "/authorize", TokenURL: f.srv.URL + "/token"},
	}}
}

// startOAuth runs the start route and returns the state cookie and the state that went to the provider.
func startOAuth(t *testing.T, s *Server, f *fakeOAuth, r *http.Request) (*http.Cookie, string) {
	t.Helper()
	rec := serve(s, r)
	if rec.Code != http.StatusFound {
		t.Fatalf("start: status %d, body %s", rec.Code, rec.Body.String())
	}
	loc, err := url.Parse(rec.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	f.challenge = loc.Query().Get("code_challenge")
	f.mu.Unlock()
	for _, c := range rec.Result().Cookies() {
		if c.Name == oauthCookie {
			return c, loc.Query().Get("state")
		}
	}
	t.Fatal("start set no state cookie")
	return nil, ""
}

// callback runs the callback route with the state cookie, the state and the code.
func callback(s *Server, cookie *http.Cookie, state, code string, session string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodGet, "/api/v1/auth/github/callback?state="+url.QueryEscape(state)+"&code="+code, nil)
	if cookie != nil {
		r.AddCookie(cookie)
	}
	if session != "" {
		r.AddCookie(&http.Cookie{Name: sessionCookie, Value: session})
	}
	return serve(s, r)
}

// newOAuthServer makes a test Server with a database and the fake provider as "github".
func newOAuthServer(t *testing.T, accountID string) (*Server, *fakeOAuth, *pgxpool.Pool) {
	s, pool := newAuthServer(t)
	f := newFakeOAuth(t, accountID)
	s.providers = map[string]*oauthProvider{"github": f.provider()}
	return s, f, pool
}

func TestOAuthStartSendsStateAndAnS256Challenge(t *testing.T) {
	s, f, _ := newOAuthServer(t, "123")
	rec := serve(s, httptest.NewRequest(http.MethodGet, "/api/v1/auth/github/start?return=/presets", nil))
	loc, _ := url.Parse(rec.Header().Get("Location"))
	q := loc.Query()

	if rec.Code != http.StatusFound || !strings.HasPrefix(loc.String(), f.srv.URL+"/authorize") {
		t.Fatalf("status %d, Location %s, want 302 to the provider", rec.Code, loc)
	}
	if q.Get("state") == "" || q.Get("code_challenge") == "" || q.Get("code_challenge_method") != "S256" {
		t.Errorf("authorize query = %v, want state and an S256 challenge", q)
	}
	c := rec.Result().Cookies()[0]
	if c.Name != oauthCookie || c.Value != q.Get("state") || !c.Secure || !c.HttpOnly || c.SameSite != http.SameSiteLaxMode || c.MaxAge != 600 {
		t.Errorf("state cookie = %+v", c)
	}
}

func TestOAuthSignInMakesASessionAndGoesBack(t *testing.T) {
	s, f, _ := newOAuthServer(t, "123")
	cookie, state := startOAuth(t, s, f, httptest.NewRequest(http.MethodGet, "/api/v1/auth/github/start?return=/presets/new", nil))

	rec := callback(s, cookie, state, "good-code", "")
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "http://localhost:3000/presets/new" {
		t.Fatalf("status %d, Location %q, body %s", rec.Code, rec.Header().Get("Location"), rec.Body.String())
	}
	var token string
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookie {
			token = c.Value
		}
	}
	if serve(s, withSession(http.MethodGet, "/api/v1/me", token)).Code != http.StatusOK {
		t.Error("the new session must work")
	}
	id, err := s.DB.FindUserByIdentity(context.Background(), db.FindUserByIdentityParams{Provider: "github", ProviderUserID: "123"})
	if err != nil || id == 0 {
		t.Errorf("no user for github account 123: %v", err)
	}
}

func TestOAuthCallbackRefusesForgedReusedExpiredAndOtherBrowserStates(t *testing.T) {
	s, f, pool := newOAuthServer(t, "123")
	start := func() (*http.Cookie, string) {
		return startOAuth(t, s, f, httptest.NewRequest(http.MethodGet, "/api/v1/auth/github/start", nil))
	}

	cookie, state := start()
	if rec := callback(s, nil, state, "good-code", ""); rec.Code != http.StatusBadRequest {
		t.Errorf("no state cookie (another browser): status %d, want 400", rec.Code)
	}
	if rec := callback(s, cookie, "forged", "good-code", ""); rec.Code != http.StatusBadRequest {
		t.Errorf("state that does not match the cookie: status %d, want 400", rec.Code)
	}

	cookie, state = start()
	if rec := callback(s, cookie, state, "good-code", ""); rec.Code != http.StatusSeeOther {
		t.Fatalf("first use: status %d", rec.Code)
	}
	if rec := callback(s, cookie, state, "good-code", ""); rec.Code != http.StatusBadRequest {
		t.Errorf("second use of one state: status %d, want 400", rec.Code)
	}

	cookie, state = start()
	if _, err := pool.Exec(context.Background(), "UPDATE oauth_flows SET expires_at = now() - interval '1 second'"); err != nil {
		t.Fatal(err)
	}
	if rec := callback(s, cookie, state, "good-code", ""); rec.Code != http.StatusBadRequest {
		t.Errorf("expired state: status %d, want 400", rec.Code)
	}
}

func TestOAuthCallbackWithAWrongVerifierOrCodeSignsNobodyIn(t *testing.T) {
	s, f, _ := newOAuthServer(t, "123")
	cookie, state := startOAuth(t, s, f, httptest.NewRequest(http.MethodGet, "/api/v1/auth/github/start", nil))
	f.mu.Lock()
	f.challenge = "a-challenge-for-another-verifier"
	f.mu.Unlock()

	rec := callback(s, cookie, state, "good-code", "")
	if rec.Code != http.StatusSeeOther || !strings.HasSuffix(rec.Header().Get("Location"), "/signin?error=provider") {
		t.Errorf("status %d, Location %q, want the sign-in page with an error", rec.Code, rec.Header().Get("Location"))
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookie {
			t.Error("a failed exchange must not make a session")
		}
	}
}

func TestOAuthReturnPathCannotLeaveTheSite(t *testing.T) {
	s, f, _ := newOAuthServer(t, "123")
	cookie, state := startOAuth(t, s, f, httptest.NewRequest(http.MethodGet, "/api/v1/auth/github/start?return="+url.QueryEscape("//evil.example/x"), nil))

	if loc := callback(s, cookie, state, "good-code", "").Header().Get("Location"); loc != "http://localhost:3000/" {
		t.Errorf("Location = %q, want the site's home page", loc)
	}
}

func TestOAuthLinkAddsAProviderOnlyToTheSignedInUser(t *testing.T) {
	s, f, _ := newOAuthServer(t, "777")
	alice := signInAs(t, s, "alice")

	if rec := serve(s, httptest.NewRequest(http.MethodGet, "/api/v1/auth/github/start?link=1", nil)); rec.Code != http.StatusUnauthorized {
		t.Errorf("link while signed out: status %d, want 401", rec.Code)
	}

	cookie, state := startOAuth(t, s, f, withSession(http.MethodGet, "/api/v1/auth/github/start?link=1", alice))
	rec := callback(s, cookie, state, "good-code", alice)
	if loc := rec.Header().Get("Location"); loc != "http://localhost:3000/settings" {
		t.Fatalf("link: Location %q, want /settings", loc)
	}
	aliceID, _ := s.signIn(context.Background(), "fake", "alice", "")
	githubUser, _ := s.signIn(context.Background(), "github", "777", "")
	if githubUser != aliceID {
		t.Errorf("github 777 belongs to user %d, want alice (%d)", githubUser, aliceID)
	}

	// Mallory tries to link the same GitHub account: it stays with alice.
	mallory := signInAs(t, s, "mallory")
	cookie, state = startOAuth(t, s, f, withSession(http.MethodGet, "/api/v1/auth/github/start?link=1", mallory))
	if loc := callback(s, cookie, state, "good-code", mallory).Header().Get("Location"); !strings.HasSuffix(loc, "/settings?error=linked-elsewhere") {
		t.Errorf("Location %q, want linked-elsewhere", loc)
	}
	if again, _ := s.signIn(context.Background(), "github", "777", ""); again != aliceID {
		t.Error("an account linked to alice must never move to mallory")
	}
}

func TestOAuthLinkNeedsTheSameUserAtTheCallback(t *testing.T) {
	s, f, _ := newOAuthServer(t, "888")
	alice, mallory := signInAs(t, s, "alice"), signInAs(t, s, "mallory")
	cookie, state := startOAuth(t, s, f, withSession(http.MethodGet, "/api/v1/auth/github/start?link=1", alice))

	if loc := callback(s, cookie, state, "good-code", mallory).Header().Get("Location"); !strings.HasSuffix(loc, "/signin?error=link-session") {
		t.Errorf("Location %q, want link-session", loc)
	}
}

func TestOAuthUnknownOrUnsetProviderIs404(t *testing.T) {
	s := newTestServer() // no OAuth apps in the settings, so no providers
	for _, path := range []string{"/api/v1/auth/github/start", "/api/v1/auth/discord/callback", "/api/v1/auth/google/start"} {
		if rec := serve(s, httptest.NewRequest(http.MethodGet, path, nil)); rec.Code != http.StatusNotFound {
			t.Errorf("%s: status %d, want 404", path, rec.Code)
		}
	}
}

func TestParseAccountIDTakesNumbersAndDigitStrings(t *testing.T) {
	for raw, want := range map[string]string{`123`: "123", `"80351110224678912"`: "80351110224678912"} {
		if got, err := parseAccountID(json.RawMessage(raw)); err != nil || got != want {
			t.Errorf("parseAccountID(%s) = %q, %v", raw, got, err)
		}
	}
	for _, raw := range []string{`null`, `"abc"`, `-1`, `1.5`, `""`} {
		if _, err := parseAccountID(json.RawMessage(raw)); err == nil {
			t.Errorf("parseAccountID(%s) must fail", raw)
		}
	}
}

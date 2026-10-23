package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/dotatlas/printing-profiles/api/internal/db"
	"github.com/dotatlas/printing-profiles/api/internal/dbtest"
)

// newAuthServer makes a test Server with a fresh, migrated database, and returns the pool for direct checks.
func newAuthServer(t *testing.T) (*Server, *pgxpool.Pool) {
	pool := dbtest.New(t)
	s := newTestServer()
	s.DB = db.New(pool)
	return s, pool
}

// signInAs signs in through the fake provider and returns the session token from the cookie.
func signInAs(t *testing.T, s *Server, login string) string {
	t.Helper()
	rec := serve(s, httptest.NewRequest(http.MethodGet, "/api/v1/auth/fake/start?login="+login, nil))
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookie && c.Value != "" {
			return c.Value
		}
	}
	t.Fatalf("fake sign-in as %s: status %d, no session cookie", login, rec.Code)
	return ""
}

// withSession returns a request with the session cookie set to token. A write also gets the site's Origin, as from the site.
func withSession(method, path, token string) *http.Request {
	r := httptest.NewRequest(method, path, nil)
	r.AddCookie(&http.Cookie{Name: sessionCookie, Value: token})
	if method != http.MethodGet {
		r.Header.Set("Origin", "http://localhost:3000")
	}
	return r
}

func TestSafeReturnPathAllowsOnlyPathsOnTheSite(t *testing.T) {
	cases := map[string]string{
		"/presets/new?x=1":     "/presets/new?x=1",
		"/":                    "/",
		"":                     "/",
		"presets":              "/",
		"//evil.example/x":     "/",
		`/\evil.example`:       "/",
		"/a\\b":                "/",
		"https://evil.example": "/",
		"/a\nb":                "/",
		"/" + string(bytes.Repeat([]byte("a"), 200)): "/",
	}
	for in, want := range cases {
		if got := safeReturnPath(in); got != want {
			t.Errorf("safeReturnPath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFakeSignInSetsAHostOnlySessionCookieAndKeepsOnlyItsHash(t *testing.T) {
	s, pool := newAuthServer(t)
	rec := serve(s, httptest.NewRequest(http.MethodGet, "/api/v1/auth/fake/start?login=alice&return=/presets", nil))

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "http://localhost:3000/presets" {
		t.Fatalf("status %d, Location %q, want 303 to the site", rec.Code, rec.Header().Get("Location"))
	}
	var c *http.Cookie
	for _, k := range rec.Result().Cookies() {
		if k.Name == sessionCookie {
			c = k
		}
	}
	if c == nil || !c.Secure || !c.HttpOnly || c.SameSite != http.SameSiteLaxMode || c.Path != "/" || c.Domain != "" || c.MaxAge != 30*24*3600 {
		t.Fatalf("cookie = %+v, want __Host- rules: Secure, HttpOnly, Lax, Path /, no Domain, 30 days", c)
	}
	var stored []byte
	if err := pool.QueryRow(context.Background(), "SELECT token_hash FROM sessions").Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(stored, hashToken(c.Value)) || bytes.Contains(stored, []byte(c.Value)) {
		t.Error("the database must keep the SHA-256 of the token, not the token")
	}
}

func TestFakeSignInDoesNotExistInProd(t *testing.T) {
	s := newTestServer()
	s.Cfg.Env = "prod"
	if rec := serve(s, httptest.NewRequest(http.MethodGet, "/api/v1/auth/fake/start?login=alice", nil)); rec.Code != http.StatusNotFound {
		t.Errorf("status %d, want 404", rec.Code)
	}
}

func TestMeNeedsASessionAndShowsOnboarding(t *testing.T) {
	s, _ := newAuthServer(t)
	if rec := serve(s, httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)); rec.Code != http.StatusUnauthorized {
		t.Errorf("signed out: status %d, want 401", rec.Code)
	}
	if rec := serve(s, withSession(http.MethodGet, "/api/v1/me", "not-a-real-token")); rec.Code != http.StatusUnauthorized {
		t.Errorf("unknown token: status %d, want 401", rec.Code)
	}

	rec := serve(s, withSession(http.MethodGet, "/api/v1/me", signInAs(t, s, "alice")))
	var me meJSON
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &me) != nil || !me.NeedsOnboarding || me.Handle != "" {
		t.Errorf("status %d, body %s, want 200 with needs_onboarding", rec.Code, rec.Body.String())
	}
}

func TestUsersAreFoundByProviderAccountNeverByEmail(t *testing.T) {
	s, _ := newAuthServer(t)
	ctx := context.Background()
	first, err := s.signIn(ctx, "github", "42", "same@example.test")
	if err != nil {
		t.Fatal(err)
	}
	again, _ := s.signIn(ctx, "github", "42", "changed@example.test")
	other, _ := s.signIn(ctx, "discord", "42", "same@example.test")

	if again != first {
		t.Errorf("the same GitHub account made a second user: %d and %d", first, again)
	}
	if other == first {
		t.Error("a Discord account with the same email joined the GitHub user; accounts must never merge by email")
	}
}

func TestSignOutEndsOnlyThisSessionAndSignOutAllEndsEvery(t *testing.T) {
	s, _ := newAuthServer(t)
	a, b := signInAs(t, s, "alice"), signInAs(t, s, "alice")

	if rec := serve(s, withSession(http.MethodPost, "/api/v1/auth/signout", a)); rec.Code != http.StatusNoContent {
		t.Fatalf("signout: status %d", rec.Code)
	}
	if serve(s, withSession(http.MethodGet, "/api/v1/me", a)).Code != http.StatusUnauthorized || serve(s, withSession(http.MethodGet, "/api/v1/me", b)).Code != http.StatusOK {
		t.Fatal("signout must end only the session that sent it")
	}

	c := signInAs(t, s, "alice")
	serve(s, withSession(http.MethodPost, "/api/v1/auth/signout-all", b))
	if serve(s, withSession(http.MethodGet, "/api/v1/me", b)).Code != http.StatusUnauthorized || serve(s, withSession(http.MethodGet, "/api/v1/me", c)).Code != http.StatusUnauthorized {
		t.Error("signout-all must end every session of the user")
	}
}

func TestSignOutOthersKeepsOnlyThisSession(t *testing.T) {
	s, _ := newAuthServer(t)
	here, phone, laptop := signInAs(t, s, "alice"), signInAs(t, s, "alice"), signInAs(t, s, "alice")
	bob := signInAs(t, s, "bob")

	rec := serve(s, withSession(http.MethodPost, "/api/v1/auth/signout-others", here))
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != `{"signed_out":2}` {
		t.Fatalf("status %d, body %s, want 200 with 2 signed out", rec.Code, rec.Body.String())
	}
	if serve(s, withSession(http.MethodGet, "/api/v1/me", here)).Code != http.StatusOK {
		t.Error("this session must stay signed in")
	}
	for name, token := range map[string]string{"phone": phone, "laptop": laptop} {
		if serve(s, withSession(http.MethodGet, "/api/v1/me", token)).Code != http.StatusUnauthorized {
			t.Errorf("the %s session must be signed out", name)
		}
	}
	if serve(s, withSession(http.MethodGet, "/api/v1/me", bob)).Code != http.StatusOK {
		t.Error("another user's session must not change")
	}
	signedOut := httptest.NewRequest(http.MethodPost, "/api/v1/auth/signout-others", nil)
	signedOut.Header.Set("Origin", "http://localhost:3000")
	if code := serve(s, signedOut).Code; code != http.StatusUnauthorized {
		t.Errorf("signed out: status %d, want 401", code)
	}
}

func TestAStrangerCannotListOrEndYourSessions(t *testing.T) {
	s, pool := newAuthServer(t)
	alice := signInAs(t, s, "alice")
	mallory := signInAs(t, s, "mallory")
	var aliceSession int64
	if err := pool.QueryRow(context.Background(), "SELECT s.id FROM sessions s JOIN identities i ON i.user_id = s.user_id WHERE i.provider_user_id = 'alice'").Scan(&aliceSession); err != nil {
		t.Fatal(err)
	}
	path := "/api/v1/me/sessions/" + strconv.FormatInt(aliceSession, 10)

	if rec := serve(s, withSession(http.MethodDelete, path, mallory)); rec.Code != http.StatusNotFound {
		t.Errorf("a stranger deleting a session: status %d, want 404", rec.Code)
	}
	signedOut := httptest.NewRequest(http.MethodDelete, path, nil)
	signedOut.Header.Set("Origin", "http://localhost:3000")
	if rec := serve(s, signedOut); rec.Code != http.StatusUnauthorized {
		t.Errorf("signed out: status %d, want 401", rec.Code)
	}
	rec := serve(s, withSession(http.MethodGet, "/api/v1/me/sessions", mallory))
	var list struct{ Items []sessionJSON }
	_ = json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list.Items) != 1 || list.Items[0].ID == aliceSession || !list.Items[0].Current {
		t.Errorf("mallory's list = %+v, want only mallory's current session", list.Items)
	}
	if rec := serve(s, withSession(http.MethodDelete, path, alice)); rec.Code != http.StatusNoContent {
		t.Errorf("the owner deleting the session: status %d, want 204", rec.Code)
	}
}

func TestExpiredAndIdleSessionsAreRefused(t *testing.T) {
	s, pool := newAuthServer(t)
	ctx := context.Background()
	expired, idle := signInAs(t, s, "expired"), signInAs(t, s, "idle")
	if _, err := pool.Exec(ctx, "UPDATE sessions SET expires_at = now() - interval '1 second' WHERE token_hash = $1", hashToken(expired)); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "UPDATE sessions SET last_seen_at = now() - interval '8 days' WHERE token_hash = $1", hashToken(idle)); err != nil {
		t.Fatal(err)
	}
	for name, token := range map[string]string{"expired": expired, "idle": idle} {
		if rec := serve(s, withSession(http.MethodGet, "/api/v1/me", token)); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s session: status %d, want 401", name, rec.Code)
		}
	}
}

func TestLastSeenMovesAtMostOnceAnHour(t *testing.T) {
	s, pool := newAuthServer(t)
	ctx := context.Background()
	token := signInAs(t, s, "alice")
	lastSeen := func() time.Time {
		var at time.Time
		if err := pool.QueryRow(ctx, "SELECT last_seen_at FROM sessions WHERE token_hash = $1", hashToken(token)).Scan(&at); err != nil {
			t.Fatal(err)
		}
		return at
	}

	before := lastSeen()
	serve(s, withSession(http.MethodGet, "/api/v1/me", token))
	if !lastSeen().Equal(before) {
		t.Error("a fresh session must not be written on every request")
	}
	if _, err := pool.Exec(ctx, "UPDATE sessions SET last_seen_at = now() - interval '2 hours'"); err != nil {
		t.Fatal(err)
	}
	serve(s, withSession(http.MethodGet, "/api/v1/me", token))
	if time.Since(lastSeen()) > time.Minute {
		t.Error("a session last seen 2 hours ago must move to now")
	}
}

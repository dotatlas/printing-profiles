package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
)

// captureMail keeps every email that the API sends, so a test can read the link.
type captureMail struct {
	mu   sync.Mutex
	sent []string // "to|body"
}

// Send keeps the email. It makes captureMail a mail.Sender.
func (c *captureMail) Send(_ context.Context, to, _, body string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sent = append(c.sent, to+"|"+body)
	return nil
}

// lastToken returns the token in the last link, and the address it went to.
func (c *captureMail) lastToken(t *testing.T) (to, token string) {
	t.Helper()
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.sent) == 0 {
		t.Fatal("no email was sent")
	}
	to, body, _ := strings.Cut(c.sent[len(c.sent)-1], "|")
	m := regexp.MustCompile(`http://localhost:3000/auth/email#token=([A-Za-z0-9_-]{43})\n`).FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("no link in the email: %q", body)
	}
	return to, m[1]
}

// postJSON sends a JSON write from the site's origin, with an optional session.
func postJSON(s *Server, path, body, session string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	r.Header.Set("Origin", "http://localhost:3000")
	r.Header.Set("Content-Type", "application/json")
	if session != "" {
		r.AddCookie(&http.Cookie{Name: sessionCookie, Value: session})
	}
	return serve(s, r)
}

// newEmailServer makes a test Server with a database and a mail sender that keeps the emails.
func newEmailServer(t *testing.T) (*Server, *captureMail) {
	s, _ := newAuthServer(t)
	m := &captureMail{}
	s.Mail = m
	return s, m
}

func TestEmailStartAnswersTheSameForEveryAddressAndSendsALink(t *testing.T) {
	s, m := newEmailServer(t)
	first := postJSON(s, "/api/v1/auth/email/start", `{"email":"  Alice@Example.COM ","return":"/presets"}`, "")
	other := postJSON(s, "/api/v1/auth/email/start", `{"email":"nobody@example.com"}`, "")

	if first.Code != http.StatusAccepted || other.Code != http.StatusAccepted || first.Body.String() != other.Body.String() {
		t.Fatalf("statuses %d and %d, bodies %q and %q, want the same 202", first.Code, other.Code, first.Body, other.Body)
	}
	m.sent = m.sent[:1]
	if to, _ := m.lastToken(t); to != "alice@example.com" {
		t.Errorf("sent to %q, want the trimmed, lower-case address", to)
	}
}

func TestEmailStartRefusesWhatIsNotOnePlainAddress(t *testing.T) {
	s, m := newEmailServer(t)
	for _, body := range []string{`{"email":"no-at-sign"}`, `{"email":"Bob <bob@example.com>"}`, `{"email":"a@b"}`,
		`{"email":"a@example.com, b@example.com"}`, `{"email":"a@example.com","admin":true}`, `not json`} {
		if rec := postJSON(s, "/api/v1/auth/email/start", body, ""); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400", body, rec.Code)
		}
	}
	if len(m.sent) != 0 {
		t.Error("no email may go out for a bad request")
	}
}

func TestEmailLinkSignsInOneTimeOnly(t *testing.T) {
	s, m := newEmailServer(t)
	postJSON(s, "/api/v1/auth/email/start", `{"email":"alice@example.com","return":"/presets/new"}`, "")
	_, token := m.lastToken(t)

	rec := postJSON(s, "/api/v1/auth/email/finish", `{"token":"`+token+`"}`, "")
	var out struct{ Return string }
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &out) != nil || out.Return != "/presets/new" {
		t.Fatalf("finish: status %d, body %s", rec.Code, rec.Body.String())
	}
	if len(rec.Result().Cookies()) == 0 || rec.Result().Cookies()[0].Name != sessionCookie {
		t.Error("finish must set the session cookie")
	}
	if again := postJSON(s, "/api/v1/auth/email/finish", `{"token":"`+token+`"}`, ""); again.Code != http.StatusBadRequest {
		t.Errorf("second use: status %d, want 400", again.Code)
	}
	if wrong := postJSON(s, "/api/v1/auth/email/finish", `{"token":"made-up"}`, ""); wrong.Code != http.StatusBadRequest {
		t.Errorf("a made-up token: status %d, want 400", wrong.Code)
	}
}

func TestEmailLinkExpiresAfter15Minutes(t *testing.T) {
	s, pool := newAuthServer(t)
	m := &captureMail{}
	s.Mail = m
	postJSON(s, "/api/v1/auth/email/start", `{"email":"alice@example.com"}`, "")
	_, token := m.lastToken(t)
	if _, err := pool.Exec(context.Background(), "UPDATE email_tokens SET expires_at = now() - interval '1 second'"); err != nil {
		t.Fatal(err)
	}
	if rec := postJSON(s, "/api/v1/auth/email/finish", `{"token":"`+token+`"}`, ""); rec.Code != http.StatusBadRequest {
		t.Errorf("expired link: status %d, want 400", rec.Code)
	}
}

func TestEmailSignInNeverJoinsAnotherProvidersUser(t *testing.T) {
	s, m := newEmailServer(t)
	githubUser, _ := s.signIn(context.Background(), "github", "5", "alice@example.com")
	postJSON(s, "/api/v1/auth/email/start", `{"email":"alice@example.com"}`, "")
	_, token := m.lastToken(t)
	postJSON(s, "/api/v1/auth/email/finish", `{"token":"`+token+`"}`, "")

	emailUser, err := s.signIn(context.Background(), "email", "alice@example.com", "alice@example.com")
	if err != nil || emailUser == githubUser {
		t.Errorf("the email sign-in joined the GitHub user (%d); accounts must never merge by email", githubUser)
	}
}

func TestEmailStartIsRateLimitedPerAddress(t *testing.T) {
	s, m := newEmailServer(t)
	for i := 0; i < 5; i++ {
		postJSON(s, "/api/v1/auth/email/start", `{"email":"alice@example.com"}`, "")
	}
	rec := postJSON(s, "/api/v1/auth/email/start", `{"email":"alice@example.com"}`, "")
	if rec.Code != http.StatusTooManyRequests || len(m.sent) != 5 {
		t.Errorf("6th request: status %d, %d emails sent, want 429 and 5 emails", rec.Code, len(m.sent))
	}
}

func TestEmailSignInIsOffWithoutASender(t *testing.T) {
	s, _ := newAuthServer(t)
	if rec := postJSON(s, "/api/v1/auth/email/start", `{"email":"alice@example.com"}`, ""); rec.Code != http.StatusServiceUnavailable {
		t.Errorf("no sender: status %d, want 503", rec.Code)
	}
}

func TestEmailSignInNeverLogsTheAddressOrTheToken(t *testing.T) {
	s, m := newEmailServer(t)
	var logs bytes.Buffer
	s.Log = slog.New(slog.NewJSONHandler(&logs, nil))
	postJSON(s, "/api/v1/auth/email/start", `{"email":"secret.person@example.com"}`, "")
	_, token := m.lastToken(t)
	postJSON(s, "/api/v1/auth/email/finish", `{"token":"`+token+`"}`, "")

	if strings.Contains(logs.String(), "secret.person") || strings.Contains(logs.String(), token) {
		t.Errorf("the log holds the address or the token:\n%s", logs.String())
	}
}

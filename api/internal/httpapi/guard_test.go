package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWritesFromAnotherOriginOrWithoutJSONAreRefused(t *testing.T) {
	s, _ := newAuthServer(t)
	token := signInAs(t, s, "alice")
	write := func(origin, contentType, body string) int {
		r := httptest.NewRequest(http.MethodPost, "/api/v1/auth/signout", strings.NewReader(body))
		r.AddCookie(&http.Cookie{Name: sessionCookie, Value: token})
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		if contentType != "" {
			r.Header.Set("Content-Type", contentType)
		}
		return serve(s, r).Code
	}

	if code := write("", "", ""); code != http.StatusForbidden {
		t.Errorf("no Origin: status %d, want 403", code)
	}
	if code := write("https://evil.example", "", ""); code != http.StatusForbidden {
		t.Errorf("another Origin: status %d, want 403", code)
	}
	if code := write("http://localhost:3000", "application/x-www-form-urlencoded", "a=1"); code != http.StatusUnsupportedMediaType {
		t.Errorf("a form body: status %d, want 415", code)
	}
	if code := write("http://localhost:3000", "text/plain", "{}"); code != http.StatusUnsupportedMediaType {
		t.Errorf("a text/plain body (a simple CORS request): status %d, want 415", code)
	}
	if serve(s, withSession(http.MethodGet, "/api/v1/me", token)).Code != http.StatusOK {
		t.Fatal("the refused writes must not have signed alice out")
	}
	if code := write("http://localhost:3000", "application/json; charset=utf-8", "{}"); code != http.StatusNoContent {
		t.Errorf("a JSON write from the site: status %d, want 204", code)
	}
}

func TestReadsNeedNoOrigin(t *testing.T) {
	if code := serve(newTestServer(), httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)).Code; code != http.StatusOK {
		t.Errorf("GET without Origin: status %d, want 200", code)
	}
}

func TestClientIPTrustsOnlyTheEntriesThatTheProxiesAdd(t *testing.T) {
	s := newTestServer()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "10.0.0.9:5555"
	r.Header.Add("X-Forwarded-For", "6.6.6.6, 203.0.113.7")
	r.Header.Add("X-Forwarded-For", "198.51.100.2")

	cases := map[int]string{0: "10.0.0.9", 1: "198.51.100.2", 2: "203.0.113.7", 5: "10.0.0.9"}
	for hops, want := range cases {
		s.Cfg.ProxyHops = hops
		if got := s.clientIP(r); got != want {
			t.Errorf("ProxyHops %d: clientIP = %q, want %q", hops, got, want)
		}
	}
	r.Header.Set("X-Forwarded-For", "not-an-ip")
	s.Cfg.ProxyHops = 1
	if got := s.clientIP(r); got != "10.0.0.9" {
		t.Errorf("a broken header: clientIP = %q, want the socket address", got)
	}
}

func TestOAuthStartIsRateLimitedPerIP(t *testing.T) {
	s, f, pool := newOAuthServer(t, "1")
	_ = f
	start := func(ip string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodGet, "/api/v1/auth/github/start", nil)
		r.RemoteAddr = ip + ":1234"
		return serve(s, r)
	}
	for i := 0; i < 30; i++ {
		if code := start("192.0.2.1").Code; code != http.StatusFound {
			t.Fatalf("request %d: status %d, want 302", i+1, code)
		}
	}
	rec := start("192.0.2.1")
	if rec.Code != http.StatusTooManyRequests || rec.Header().Get("Retry-After") == "" {
		t.Errorf("request 31: status %d, Retry-After %q, want 429 with Retry-After", rec.Code, rec.Header().Get("Retry-After"))
	}
	if code := start("192.0.2.2").Code; code != http.StatusFound {
		t.Errorf("another IP: status %d, want 302", code)
	}

	var plain int
	if err := pool.QueryRow(t.Context(), "SELECT count(*) FROM rate_limits WHERE position('192.0.2.1'::bytea in key) > 0").Scan(&plain); err != nil || plain != 0 {
		t.Errorf("the table must not hold the plain IP (rows with it: %d, err %v)", plain, err)
	}
}

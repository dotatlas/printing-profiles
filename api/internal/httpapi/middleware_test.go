package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
)

// serve sends one request through the full Handler and returns the answer. The tests below use it.
func serve(s *Server, r *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, r)
	return rec
}

func TestEveryAnswerHasARandomRequestID(t *testing.T) {
	s := newTestServer()
	first := serve(s, httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)).Header().Get("X-Request-ID")
	second := serve(s, httptest.NewRequest(http.MethodGet, "/api/v1/nothing", nil)).Header().Get("X-Request-ID")

	hex32 := regexp.MustCompile(`^[0-9a-f]{32}$`)
	if !hex32.MatchString(first) || !hex32.MatchString(second) {
		t.Fatalf("request IDs %q and %q are not 32 hex characters", first, second)
	}
	if first == second {
		t.Errorf("two requests got the same ID %q", first)
	}
}

func TestEveryRequestIsLoggedWithItsID(t *testing.T) {
	var logs bytes.Buffer
	s := newTestServer()
	s.Log = slog.New(slog.NewJSONHandler(&logs, nil))

	rec := serve(s, httptest.NewRequest(http.MethodGet, "/api/v1/health", nil))

	var line struct {
		Msg, Method, Path string
		Status            int
	}
	if err := json.Unmarshal(logs.Bytes(), &line); err != nil {
		t.Fatalf("log is not one JSON line: %v\n%s", err, logs.String())
	}
	var raw map[string]any
	_ = json.Unmarshal(logs.Bytes(), &raw)
	if raw["request_id"] != rec.Header().Get("X-Request-ID") {
		t.Errorf("logged request_id %v, answer header %q", raw["request_id"], rec.Header().Get("X-Request-ID"))
	}
	if line.Msg != "request" || line.Method != "GET" || line.Path != "/api/v1/health" || line.Status != 200 {
		t.Errorf("log line = %s", logs.String())
	}
	if _, ok := raw["duration_ms"]; !ok {
		t.Errorf("log line has no duration_ms: %s", logs.String())
	}
}

func TestPanicBecomesProblem500AndIsLogged(t *testing.T) {
	var logs bytes.Buffer
	s := newTestServer()
	s.Log = slog.New(slog.NewJSONHandler(&logs, nil))
	boom := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") })
	rec := httptest.NewRecorder()

	requestID(s.recoverPanics(boom)).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if p := decodeProblem(t, rec); p.Detail != "" {
		t.Errorf("a 500 must not show details, got %q", p.Detail)
	}
	if !strings.Contains(logs.String(), "boom") || !strings.Contains(logs.String(), rec.Header().Get("X-Request-ID")) {
		t.Errorf("panic log misses the value or the request ID: %s", logs.String())
	}
}

func TestEveryAnswerHasSecurityHeaders(t *testing.T) {
	rec := serve(newTestServer(), httptest.NewRequest(http.MethodGet, "/api/v1/nothing", nil))

	want := map[string]string{
		"X-Content-Type-Options":    "nosniff",
		"Content-Security-Policy":   "default-src 'none'; frame-ancestors 'none'",
		"Referrer-Policy":           "no-referrer",
		"Strict-Transport-Security": "max-age=63072000; includeSubDomains",
	}
	for name, value := range want {
		if got := rec.Header().Get(name); got != value {
			t.Errorf("%s = %q, want %q", name, got, value)
		}
	}
}

func TestBodiesOverOneMiBCannotBeRead(t *testing.T) {
	var readErr error
	readAll := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, readErr = io.ReadAll(r.Body) })
	big := strings.NewReader(strings.Repeat("x", maxBodyBytes+1))

	limitBody(readAll).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", big))

	var tooBig *http.MaxBytesError
	if !errors.As(readErr, &tooBig) {
		t.Errorf("read error = %v, want *http.MaxBytesError", readErr)
	}
}

func TestCORSAllowsOnlyTheSiteOrigin(t *testing.T) {
	s := newTestServer()

	site := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	site.Header.Set("Origin", "http://localhost:3000")
	rec := serve(s, site)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:3000" {
		t.Errorf("site origin: Allow-Origin = %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("site origin: Allow-Credentials = %q", got)
	}

	foreign := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	foreign.Header.Set("Origin", "https://evil.example")
	rec = serve(s, foreign)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("foreign origin got Allow-Origin %q, want none", got)
	}
	if got := rec.Header().Get("Vary"); !strings.Contains(got, "Origin") {
		t.Errorf("Vary = %q, want it to contain Origin", got)
	}
}

func TestCORSPreflightFromTheSite(t *testing.T) {
	pre := httptest.NewRequest(http.MethodOptions, "/api/v1/health", nil)
	pre.Header.Set("Origin", "http://localhost:3000")
	pre.Header.Set("Access-Control-Request-Method", "POST")

	rec := serve(newTestServer(), pre)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); got != "GET, POST, PATCH, DELETE" {
		t.Errorf("Allow-Methods = %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Headers"); got != "Content-Type" {
		t.Errorf("Allow-Headers = %q", got)
	}
	if got := rec.Header().Get("Access-Control-Max-Age"); got != "600" {
		t.Errorf("Max-Age = %q", got)
	}
}

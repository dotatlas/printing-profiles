package httpapi

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
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

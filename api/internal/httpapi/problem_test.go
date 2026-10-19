package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// decodeProblem reads a problem details body, and fails the test when the body is not one.
func decodeProblem(t *testing.T, rec *httptest.ResponseRecorder) problem {
	t.Helper()
	if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Fatalf("Content-Type = %q, want application/problem+json", ct)
	}
	var p problem
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("body is not JSON: %v", err)
	}
	return p
}

func TestUnknownRouteIsProblem404(t *testing.T) {
	rec := serve(newTestServer(), httptest.NewRequest(http.MethodGet, "/api/v1/nothing", nil))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	p := decodeProblem(t, rec)
	if p.Status != 404 || p.Title != "Not Found" || p.Type != "about:blank" {
		t.Errorf("problem = %+v", p)
	}
}

func TestWrongMethodIsProblem405WithAllow(t *testing.T) {
	rec := serve(newTestServer(), httptest.NewRequest(http.MethodPost, "/api/v1/health", nil))

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
	if allow := rec.Header().Get("Allow"); !strings.Contains(allow, "GET") {
		t.Errorf("Allow = %q, want it to list GET", allow)
	}
	if p := decodeProblem(t, rec); p.Status != 405 {
		t.Errorf("problem status = %d, want 405", p.Status)
	}
}

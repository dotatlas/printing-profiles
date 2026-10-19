package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dotatlas/printing-profiles/api/internal/config"
)

// newTestServer makes a Server with dev settings and a logger that writes nowhere. Most tests use it.
func newTestServer() *Server {
	return &Server{
		Log: slog.New(slog.NewJSONHandler(io.Discard, nil)),
		Cfg: config.Config{Env: "dev", Port: 8080, SiteOrigin: "http://localhost:3000"},
	}
}

func TestHealthReturnsOK(t *testing.T) {
	rec := httptest.NewRecorder()

	newTestServer().Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/health", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	var body struct{ Status string }
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not JSON: %v", err)
	}
	if body.Status != "ok" {
		t.Errorf("status field = %q, want ok", body.Status)
	}
}

func TestServeStopsCleanlyWhenContextIsCanceled(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Serve(ctx, ln, newTestServer().Handler()) }()

	resp, err := http.Get("http://" + ln.Addr().String() + "/api/v1/health")
	if err != nil {
		t.Fatalf("server does not answer: %v", err)
	}
	resp.Body.Close()
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Serve returned %v, want nil after a clean stop", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not stop after the context was canceled")
	}
}

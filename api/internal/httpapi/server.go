// Package httpapi holds the HTTP side of the API: the routes, the middleware and the server loop.
package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/dotatlas/printing-profiles/api/internal/config"
	"github.com/dotatlas/printing-profiles/api/internal/db"
)

// Server holds what the handlers need. main makes one and gives its Handler to Serve.
type Server struct {
	Log *slog.Logger
	Cfg config.Config
	DB  *db.Queries // the sqlc queries; nil in tests that do not touch the database
}

// Handler returns all routes inside the middleware. Serve runs it, and the tests call it directly.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/health", s.handleHealth)
	mux.HandleFunc("GET /api/v1/printers", s.handlePrinters)
	mux.HandleFunc("GET /api/v1/materials", s.handleMaterials)
	mux.HandleFunc("GET /api/v1/slicers", s.handleSlicers)

	// The last wrapper runs first. requestID is last, so every answer and log line has an ID.
	var h http.Handler = problemsForUnmatched(mux)
	h = limitBody(h)
	h = s.cors(h)
	h = securityHeaders(h)
	h = s.recoverPanics(h)
	h = s.logRequests(h)
	h = requestID(h)
	return h
}

// handleHealth tells the caller that the API is up. The router calls it for GET /api/v1/health.
func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// writeJSON sends v as a JSON body with the status. All handlers use it for their answers.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	// The status is already sent, so an encode error cannot change the answer.
	_ = json.NewEncoder(w).Encode(v)
}

// Serve runs h on ln until ctx stops, then lets open requests finish. main calls it after it opens the port.
func Serve(ctx context.Context, ln net.Listener, h http.Handler) error {
	// ReadHeaderTimeout stops a slow client from keeping a connection open forever.
	srv := &http.Server{Handler: h, ReadHeaderTimeout: 10 * time.Second}
	errc := make(chan error, 1) // One slot, so the goroutine can always send and exit.
	go func() { errc <- srv.Serve(ln) }()

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	// Give open requests 5 seconds. Make a new context, because ctx is already done.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

// Package httpapi holds the HTTP side of the API: the routes, the middleware and the server loop.
package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/dotatlas/printing-profiles/api/internal/config"
)

// Server holds what the handlers need. main makes one and serves its Handler.
type Server struct {
	Log *slog.Logger
	Cfg config.Config
}

// Handler returns the router with all routes. main serves it, and the tests call it directly.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/health", s.handleHealth)
	return mux
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

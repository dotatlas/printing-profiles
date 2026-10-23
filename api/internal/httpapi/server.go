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
	"github.com/dotatlas/printing-profiles/api/internal/mail"
)

// Server holds what the handlers need. main makes one and gives its Handler to Serve.
type Server struct {
	Log  *slog.Logger
	Cfg  config.Config
	DB   *db.Queries // the sqlc queries; nil in tests that do not touch the database
	Mail mail.Sender // the sender of sign-in emails; nil means no email sign-in
	// Uploads is the private bucket that the browser sends files to, and Files the public bucket of checked files.
	// With either nil, the upload routes answer 503.
	Uploads FileStore
	Files   FileStore

	providers map[string]*oauthProvider // the OAuth providers; Handler makes them from Cfg, and tests put fakes here
}

// Handler returns all routes inside the middleware. Serve runs it, and the tests call it directly.
func (s *Server) Handler() http.Handler {
	if s.providers == nil {
		s.providers = providersFrom(s.Cfg)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/health", s.handleHealth)
	mux.HandleFunc("GET /api/v1/printers", s.handlePrinters)
	mux.HandleFunc("GET /api/v1/materials", s.handleMaterials)
	mux.HandleFunc("GET /api/v1/slicers", s.handleSlicers)
	mux.HandleFunc("GET /api/v1/me", s.handleMe)
	mux.HandleFunc("GET /api/v1/me/sessions", s.handleSessions)
	mux.HandleFunc("DELETE /api/v1/me/sessions/{id}", s.handleDeleteSession)
	mux.HandleFunc("PATCH /api/v1/me", s.handleUpdateMe)
	mux.HandleFunc("POST /api/v1/me/onboarding", s.handleOnboarding)
	mux.HandleFunc("GET /api/v1/me/identities", s.handleIdentities)
	mux.HandleFunc("DELETE /api/v1/me/identities/{id}", s.handleDeleteIdentity)
	mux.HandleFunc("GET /api/v1/me/presets", s.handleMyPresets)
	mux.HandleFunc("GET /api/v1/presets", s.handleListPresets)
	mux.HandleFunc("POST /api/v1/presets", s.handleCreatePreset)
	mux.HandleFunc("GET /api/v1/presets/{id}", s.handleGetPreset)
	mux.HandleFunc("PATCH /api/v1/presets/{id}", s.handleUpdatePreset)
	mux.HandleFunc("DELETE /api/v1/presets/{id}", s.handleDeletePreset)
	mux.HandleFunc("GET /api/v1/presets/{id}/download", s.handlePresetDownload)
	mux.HandleFunc("GET /api/v1/presets/{id}/reviews", s.handleListReviews)
	mux.HandleFunc("POST /api/v1/presets/{id}/reviews", s.handleSaveReview)
	mux.HandleFunc("GET /api/v1/presets/{id}/reviews/mine", s.handleMyReview)
	mux.HandleFunc("DELETE /api/v1/presets/{id}/reviews/mine", s.handleDeleteMyReview)
	mux.HandleFunc("GET /api/v1/presets/{id}/comments", s.handleListComments)
	mux.HandleFunc("POST /api/v1/presets/{id}/comments", s.handleCreateComment)
	mux.HandleFunc("PATCH /api/v1/comments/{id}", s.handleUpdateComment)
	mux.HandleFunc("DELETE /api/v1/comments/{id}", s.handleDeleteComment)
	mux.HandleFunc("POST /api/v1/uploads", s.handleCreateUpload)
	mux.HandleFunc("POST /api/v1/uploads/{id}/complete", s.handleCompleteUpload)
	mux.HandleFunc("POST /api/v1/auth/signout", s.handleSignOut)
	mux.HandleFunc("POST /api/v1/auth/signout-all", s.handleSignOutAll)
	mux.HandleFunc("POST /api/v1/auth/signout-others", s.handleSignOutOthers)
	mux.HandleFunc("GET /api/v1/auth/{provider}/start", s.handleOAuthStart)
	mux.HandleFunc("GET /api/v1/auth/{provider}/callback", s.handleOAuthCallback)
	mux.HandleFunc("POST /api/v1/auth/email/start", s.handleEmailStart)
	mux.HandleFunc("POST /api/v1/auth/email/finish", s.handleEmailFinish)
	if s.Cfg.Env == "dev" {
		// The fake provider signs in without a password or a provider, so it must never exist in prod.
		mux.HandleFunc("GET /api/v1/auth/fake/start", s.handleFakeSignIn)
		// Without R2, dev keeps files in memory, and these routes stand in for the bucket URLs.
		mux.HandleFunc(devUploadPattern, s.handleDevUpload)
		mux.HandleFunc("GET /api/v1/dev/files/{key...}", s.handleDevFile)
	}

	// The last wrapper runs first. requestID is last, so every answer and log line has an ID.
	var h http.Handler = problemsForUnmatched(mux)
	h = s.checkWrites(mux, h)
	h = s.loadSession(h)
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

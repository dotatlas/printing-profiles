package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"
)

// ctxKey is the type of the context keys in this package, so they cannot clash with keys of other packages.
type ctxKey int

// requestIDKey is the context key for the request ID. requestID sets it, and requestIDFrom reads it.
const requestIDKey ctxKey = 0

// requestID gives each request a random ID in the X-Request-ID header and in the context. Handler runs it first.
func requestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b := make([]byte, 16)
		_, _ = rand.Read(b) // In Go 1.24 and newer, crypto/rand.Read does not return an error.
		id := hex.EncodeToString(b)
		w.Header().Set("X-Request-ID", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey, id)))
	})
}

// requestIDFrom returns the request ID that requestID put in ctx, or "" when there is none. The logs use it.
func requestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

// statusWriter keeps the status that a handler sends. logRequests uses it to log the status.
type statusWriter struct {
	http.ResponseWriter
	status int
}

// WriteHeader keeps the first status and sends it on. A handler calls it through the http.ResponseWriter.
func (w *statusWriter) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
	w.ResponseWriter.WriteHeader(status)
}

// Write keeps status 200 when the handler writes a body first, as net/http does, and sends the body on.
func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

// Unwrap gives the real writer to http.ResponseController, so features like flush still work.
func (w *statusWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

// logRequests writes one JSON log line for each request after the handler is done. Handler runs it after requestID.
func (s *Server) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w}
		next.ServeHTTP(sw, r)
		if sw.status == 0 {
			sw.status = http.StatusOK // The handler wrote nothing, so net/http sends 200.
		}
		// Log no bodies and no headers, because they can contain tokens.
		s.Log.Info("request",
			"request_id", requestIDFrom(r.Context()),
			"method", r.Method,
			"path", r.URL.Path,
			"status", sw.status,
			"duration_ms", time.Since(start).Milliseconds())
	})
}

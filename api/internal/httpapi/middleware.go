package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"runtime/debug"
	"time"
)

// maxBodyBytes is the largest request body that the API reads. Upload files go to S3, not through the API.
const maxBodyBytes = 1 << 20

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

// recoverPanics changes a panic in a handler into a 500 answer and a log line, so the server continues to run.
func (s *Server) recoverPanics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			v := recover()
			if v == nil {
				return
			}
			if v == http.ErrAbortHandler {
				panic(v) // net/http uses this panic to stop an answer on purpose. Let it continue.
			}
			s.Log.Error("panic",
				"request_id", requestIDFrom(r.Context()),
				"panic", fmt.Sprint(v),
				"stack", string(debug.Stack()))
			writeProblem(w, http.StatusInternalServerError, "")
		}()
		next.ServeHTTP(w, r)
	})
}

// securityHeaders adds the browser security headers to every answer. Handler runs it before the router.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		// The API sends only JSON, so a browser must not run or frame anything from it.
		h.Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains")
		next.ServeHTTP(w, r)
	})
}

// cors lets only the site's own origin call the API from a browser, with cookies. Handler runs it before the router.
func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Add("Vary", "Origin") // Caches must keep the answers for different origins apart.
		if r.Header.Get("Origin") != s.Cfg.SiteOrigin {
			next.ServeHTTP(w, r) // No CORS headers, so the browser blocks the answer for other origins.
			return
		}
		h.Set("Access-Control-Allow-Origin", s.Cfg.SiteOrigin)
		h.Set("Access-Control-Allow-Credentials", "true")
		// A preflight asks which methods and headers are allowed, before the browser sends the real request.
		if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
			h.Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE")
			h.Set("Access-Control-Allow-Headers", "Content-Type")
			h.Set("Access-Control-Max-Age", "600")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// limitBody stops a handler from reading more than maxBodyBytes of a request body. Handler runs it before the router.
func limitBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
		next.ServeHTTP(w, r)
	})
}

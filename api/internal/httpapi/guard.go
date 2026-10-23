package httpapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"mime"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/dotatlas/printing-profiles/api/internal/db"
)

// checkWrites refuses a POST, PATCH or DELETE that does not come from the site, or that has a body that is not JSON.
// A cross-site form cannot send JSON without a CORS preflight, so this stops CSRF. Handler runs it before the router.
// A request that matches no route goes on, so it still gets the router's 404 or 405.
func (s *Server) checkWrites(mux *http.ServeMux, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}
		if _, pattern := mux.Handler(r); pattern == "" {
			next.ServeHTTP(w, r)
			return
		}
		if r.Header.Get("Origin") != s.Cfg.SiteOrigin {
			writeProblem(w, http.StatusForbidden, "this request must come from the site")
			return
		}
		if r.ContentLength != 0 {
			mt, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if err != nil || mt != "application/json" {
				writeProblem(w, http.StatusUnsupportedMediaType, "send the body as application/json")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// clientIP returns the IP address of the client. With ProxyHops 0 it is the socket address. Otherwise it is the
// X-Forwarded-For entry that the nearest trusted proxy added, never an entry that the client can write itself.
func (s *Server) clientIP(r *http.Request) string {
	if s.Cfg.ProxyHops > 0 {
		var entries []string
		for _, h := range r.Header.Values("X-Forwarded-For") {
			for _, e := range strings.Split(h, ",") {
				entries = append(entries, strings.TrimSpace(e))
			}
		}
		if i := len(entries) - s.Cfg.ProxyHops; i >= 0 && net.ParseIP(entries[i]) != nil {
			return entries[i]
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// rateLimitKey returns the HMAC of the action and the subject (an IP or an address), so the table never holds either.
func (s *Server) rateLimitKey(action, subject string) []byte {
	m := hmac.New(sha256.New, []byte(s.Cfg.HashKey))
	m.Write([]byte(action + "\x00" + subject))
	return m.Sum(nil)
}

// allow counts one request of action by subject in a fixed window, and sends 429 when the count goes past max.
// It returns false when the request must stop. The sign-in handlers call it before they do any work.
func (s *Server) allow(w http.ResponseWriter, r *http.Request, action, subject string, max int32, window time.Duration) bool {
	now := time.Now()
	start := now.Truncate(window)
	count, err := s.DB.HitRateLimit(r.Context(), db.HitRateLimitParams{Key: s.rateLimitKey(action, subject), WindowStart: start})
	if err != nil {
		s.fail(w, r, err)
		return false
	}
	if count == 1 {
		// The first request of a window is rare enough to also remove the old windows.
		if err := s.DB.DeleteOldRateLimits(r.Context()); err != nil {
			s.fail(w, r, err)
			return false
		}
	}
	if count > max {
		retry := start.Add(window).Sub(now)
		w.Header().Set("Retry-After", strconv.Itoa(int(retry.Seconds())+1))
		writeProblem(w, http.StatusTooManyRequests, "too many attempts; try again later")
		return false
	}
	return true
}

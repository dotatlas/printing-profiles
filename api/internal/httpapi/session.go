package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/dotatlas/printing-profiles/api/internal/db"
)

const (
	sessionCookie   = "__Host-session"
	sessionLifetime = 30 * 24 * time.Hour // the same 30 days as CreateSession in queries/identity.sql
	maxUserAgent    = 200                 // the longest user agent that a session row keeps
)

// sessionKey is the context key for the signed-in user. loadSession sets it, and userFrom reads it.
const sessionKey ctxKey = 1

// sessionUser is the signed-in user of a request. loadSession puts it in the context.
type sessionUser struct {
	SessionID   int64
	UserID      int64
	Handle      string // empty until onboarding
	DisplayName string
}

// newToken returns a random token for a cookie or a link, and the SHA-256 that the database keeps.
func newToken() (token string, hash []byte) {
	b := make([]byte, 32)
	_, _ = rand.Read(b) // In Go 1.24 and newer, crypto/rand.Read does not return an error.
	token = base64.RawURLEncoding.EncodeToString(b)
	return token, hashToken(token)
}

// hashToken returns the SHA-256 of a token. The database stores and looks up tokens only by this hash.
func hashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}

// userFrom returns the signed-in user that loadSession found, or false when the request has no valid session.
func userFrom(ctx context.Context) (sessionUser, bool) {
	u, ok := ctx.Value(sessionKey).(sessionUser)
	return u, ok
}

// loadSession finds the user of the session cookie and puts it in the context. Handler runs it before the router.
func (s *Server) loadSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(sessionCookie)
		if err != nil || s.DB == nil {
			next.ServeHTTP(w, r)
			return
		}
		row, err := s.DB.GetSession(r.Context(), hashToken(c.Value))
		if errors.Is(err, pgx.ErrNoRows) {
			next.ServeHTTP(w, r) // An unknown or old token is the same as no cookie.
			return
		}
		if err != nil {
			s.fail(w, r, err)
			return
		}
		if time.Since(row.LastSeenAt) > time.Hour {
			if err := s.DB.TouchSession(r.Context(), row.ID); err != nil {
				s.fail(w, r, err)
				return
			}
		}
		u := sessionUser{SessionID: row.ID, UserID: row.UserID, Handle: row.Handle, DisplayName: row.DisplayName}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), sessionKey, u)))
	})
}

// requireUser returns the signed-in user, or sends 401 and returns false. Every route for a signed-in user calls it first.
func requireUser(w http.ResponseWriter, r *http.Request) (sessionUser, bool) {
	u, ok := userFrom(r.Context())
	if !ok {
		writeProblem(w, http.StatusUnauthorized, "sign in first")
	}
	return u, ok
}

// startSession makes a new session for userID and sets its cookie. The sign-in flows call it when a sign-in succeeds.
func (s *Server) startSession(w http.ResponseWriter, r *http.Request, userID int64) error {
	ctx := r.Context()
	if err := s.DB.DeleteExpiredSessions(ctx, userID); err != nil {
		return err
	}
	token, hash := newToken()
	ua := r.UserAgent()
	if len(ua) > maxUserAgent {
		ua = ua[:maxUserAgent]
	}
	if err := s.DB.CreateSession(ctx, db.CreateSessionParams{TokenHash: hash, UserID: userID, UserAgent: ua}); err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		MaxAge:   int(sessionLifetime.Seconds()),
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	return nil
}

// clearSessionCookie tells the browser to drop the session cookie. The sign-out handlers call it.
func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Path: "/", MaxAge: -1, Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode})
}

// meJSON is the answer of handleMe.
type meJSON struct {
	Handle          string `json:"handle"`
	DisplayName     string `json:"display_name"`
	NeedsOnboarding bool   `json:"needs_onboarding"`
}

// handleMe tells the site who is signed in. The router calls it for GET /api/v1/me.
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	u, ok := requireUser(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, meJSON{Handle: u.Handle, DisplayName: u.DisplayName, NeedsOnboarding: u.Handle == ""})
}

// handleSignOut ends the session of this request. The router calls it for POST /api/v1/auth/signout.
func (s *Server) handleSignOut(w http.ResponseWriter, r *http.Request) {
	if u, ok := userFrom(r.Context()); ok {
		if _, err := s.DB.DeleteSession(r.Context(), db.DeleteSessionParams{ID: u.SessionID, UserID: u.UserID}); err != nil {
			s.fail(w, r, err)
			return
		}
	}
	clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

// handleSignOutAll ends every session of the signed-in user. The router calls it for POST /api/v1/auth/signout-all.
func (s *Server) handleSignOutAll(w http.ResponseWriter, r *http.Request) {
	u, ok := requireUser(w, r)
	if !ok {
		return
	}
	if err := s.DB.DeleteUserSessions(r.Context(), u.UserID); err != nil {
		s.fail(w, r, err)
		return
	}
	clearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

// handleSignOutOthers ends every session of the signed-in user except this one, and says how many it ended.
// The router calls it for POST /api/v1/auth/signout-others.
func (s *Server) handleSignOutOthers(w http.ResponseWriter, r *http.Request) {
	u, ok := requireUser(w, r)
	if !ok {
		return
	}
	n, err := s.DB.DeleteOtherSessions(r.Context(), db.DeleteOtherSessionsParams{UserID: u.UserID, KeepID: u.SessionID})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{"signed_out": n})
}

// sessionJSON is one session in the answer of handleSessions.
type sessionJSON struct {
	ID         int64     `json:"id"`
	UserAgent  string    `json:"user_agent"`
	CreatedAt  time.Time `json:"created_at"`
	LastSeenAt time.Time `json:"last_seen_at"`
	Current    bool      `json:"current"`
}

// handleSessions lists the signed-in user's sessions. The router calls it for GET /api/v1/me/sessions.
func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	u, ok := requireUser(w, r)
	if !ok {
		return
	}
	rows, err := s.DB.ListSessions(r.Context(), u.UserID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	items := make([]sessionJSON, 0, len(rows))
	for _, row := range rows {
		items = append(items, sessionJSON{ID: row.ID, UserAgent: row.UserAgent, CreatedAt: row.CreatedAt, LastSeenAt: row.LastSeenAt, Current: row.ID == u.SessionID})
	}
	writeJSON(w, http.StatusOK, map[string][]sessionJSON{"items": items})
}

// handleDeleteSession ends one of the signed-in user's sessions. The router calls it for DELETE /api/v1/me/sessions/{id}.
func (s *Server) handleDeleteSession(w http.ResponseWriter, r *http.Request) {
	u, ok := requireUser(w, r)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeProblem(w, http.StatusNotFound, "session not found")
		return
	}
	n, err := s.DB.DeleteSession(r.Context(), db.DeleteSessionParams{ID: id, UserID: u.UserID})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if n == 0 {
		writeProblem(w, http.StatusNotFound, "session not found")
		return
	}
	if id == u.SessionID {
		clearSessionCookie(w)
	}
	w.WriteHeader(http.StatusNoContent)
}

package httpapi

import (
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/dotatlas/printing-profiles/api/internal/db"
)

// handlePattern is the form of a handle: lowercase letters and digits, with single dashes between them.
var handlePattern = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// reservedHandles are handles that could look like the site's staff or pages. handleOnboarding refuses them.
var reservedHandles = map[string]bool{
	"admin": true, "administrator": true, "moderator": true, "mod": true, "staff": true, "support": true,
	"root": true, "system": true, "api": true, "printingprofiles": true, "printing-profiles": true,
}

// maxDisplayName is the longest display name, the same as the check in the users table.
const maxDisplayName = 60

// handleOnboarding gives a new user a handle and records that the user accepted the terms. It works one time.
// The router calls it for POST /api/v1/me/onboarding.
func (s *Server) handleOnboarding(w http.ResponseWriter, r *http.Request) {
	u, ok := requireUser(w, r)
	if !ok {
		return
	}
	var in struct {
		Handle      string `json:"handle"`
		AcceptTerms bool   `json:"accept_terms"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	handle := strings.TrimSpace(in.Handle)
	switch {
	case !in.AcceptTerms:
		writeProblem(w, http.StatusBadRequest, "accept the terms to continue")
		return
	case len(handle) < 3 || len(handle) > 30 || !handlePattern.MatchString(handle):
		writeProblem(w, http.StatusBadRequest, "a handle has 3 to 30 lowercase letters, digits and single dashes")
		return
	case reservedHandles[handle]:
		writeProblem(w, http.StatusConflict, "that handle is taken")
		return
	}
	n, err := s.DB.FinishOnboarding(r.Context(), db.FinishOnboardingParams{ID: u.UserID, Handle: pgtype.Text{String: handle, Valid: true}})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		writeProblem(w, http.StatusConflict, "that handle is taken")
		return
	}
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if n == 0 {
		writeProblem(w, http.StatusConflict, "this account already has a handle")
		return
	}
	writeJSON(w, http.StatusOK, meJSON{Handle: handle, DisplayName: u.DisplayName})
}

// handleUpdateMe changes the signed-in user's display name. The router calls it for PATCH /api/v1/me.
func (s *Server) handleUpdateMe(w http.ResponseWriter, r *http.Request) {
	u, ok := requireUser(w, r)
	if !ok {
		return
	}
	var in struct {
		DisplayName string `json:"display_name"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	name := strings.TrimSpace(in.DisplayName)
	if utf8.RuneCountInString(name) > maxDisplayName || strings.ContainsFunc(name, unicode.IsControl) {
		writeProblem(w, http.StatusBadRequest, "a display name has at most 60 characters and no control characters")
		return
	}
	if err := s.DB.SetDisplayName(r.Context(), db.SetDisplayNameParams{ID: u.UserID, DisplayName: name}); err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, meJSON{Handle: u.Handle, DisplayName: name, NeedsOnboarding: u.Handle == ""})
}

// identityJSON is one sign-in method in the answer of handleIdentities. Email is set only for the email method.
type identityJSON struct {
	ID        int64     `json:"id"`
	Provider  string    `json:"provider"`
	Email     string    `json:"email,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// handleIdentities lists the ways that the signed-in user can sign in. The router calls it for GET /api/v1/me/identities.
func (s *Server) handleIdentities(w http.ResponseWriter, r *http.Request) {
	u, ok := requireUser(w, r)
	if !ok {
		return
	}
	rows, err := s.DB.ListIdentities(r.Context(), u.UserID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	items := make([]identityJSON, 0, len(rows))
	for _, row := range rows {
		items = append(items, identityJSON{ID: row.ID, Provider: row.Provider, Email: row.Email, CreatedAt: row.CreatedAt})
	}
	writeJSON(w, http.StatusOK, map[string][]identityJSON{"items": items})
}

// handleDeleteIdentity removes one sign-in method, but never the last one, so the user can still sign in.
// The router calls it for DELETE /api/v1/me/identities/{id}.
func (s *Server) handleDeleteIdentity(w http.ResponseWriter, r *http.Request) {
	u, ok := requireUser(w, r)
	if !ok {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeProblem(w, http.StatusNotFound, "sign-in method not found")
		return
	}
	n, err := s.DB.DeleteIdentity(r.Context(), db.DeleteIdentityParams{ID: id, UserID: u.UserID})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if n == 0 {
		// Nothing was deleted. When the method is this user's own, it was the last one; otherwise it is not theirs.
		own, err := s.DB.ListIdentities(r.Context(), u.UserID)
		if err != nil {
			s.fail(w, r, err)
			return
		}
		for _, row := range own {
			if row.ID == id {
				writeProblem(w, http.StatusConflict, "this is your only way to sign in; add another one first")
				return
			}
		}
		writeProblem(w, http.StatusNotFound, "sign-in method not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

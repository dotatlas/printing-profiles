package httpapi

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/dotatlas/printing-profiles/api/internal/db"
)

// fakeLogin is the form of a test user name for the dev-only fake provider.
var fakeLogin = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// safeReturnPath returns p when it is a path on the site, or "/" when it could lead to another site.
// The sign-in flows call it on every return path, so that the API is never an open redirect.
func safeReturnPath(p string) string {
	if len(p) > 200 || !strings.HasPrefix(p, "/") || strings.HasPrefix(p, "//") || strings.HasPrefix(p, `/\`) {
		return "/"
	}
	// A browser reads a backslash or a control character in a URL in surprising ways, so refuse them.
	for _, c := range p {
		if c == '\\' || c < 0x20 || c == 0x7f {
			return "/"
		}
	}
	return p
}

// redirectToSite sends the browser to a path on the site. The sign-in flows call it at the end.
func (s *Server) redirectToSite(w http.ResponseWriter, r *http.Request, path string) {
	http.Redirect(w, r, s.Cfg.SiteOrigin+safeReturnPath(path), http.StatusSeeOther)
}

// signIn returns the user of a provider account, and makes the user when this is its first sign-in.
// It finds users only by (provider, provider user ID), never by email. The sign-in flows call it.
func (s *Server) signIn(ctx context.Context, provider, providerUserID, email string) (int64, error) {
	key := db.FindUserByIdentityParams{Provider: provider, ProviderUserID: providerUserID}
	id, err := s.DB.FindUserByIdentity(ctx, key)
	if !errors.Is(err, pgx.ErrNoRows) {
		return id, err
	}
	id, err = s.DB.CreateUserWithIdentity(ctx, db.CreateUserWithIdentityParams{Provider: provider, ProviderUserID: providerUserID, Email: email})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		// A parallel first sign-in of the same account made the user first. Use that user.
		return s.DB.FindUserByIdentity(ctx, key)
	}
	return id, err
}

// handleFakeSignIn signs in as any test user, with no provider. Handler adds this route only when ENV is dev.
// The router calls it for GET /api/v1/auth/fake/start?login=<name>&return=<path>.
func (s *Server) handleFakeSignIn(w http.ResponseWriter, r *http.Request) {
	login := r.URL.Query().Get("login")
	if len(login) > 40 || !fakeLogin.MatchString(login) {
		writeProblem(w, http.StatusBadRequest, "login must be lowercase letters, digits and dashes")
		return
	}
	userID, err := s.signIn(r.Context(), "fake", login, login+"@example.test")
	if err == nil {
		err = s.startSession(w, r, userID)
	}
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.redirectToSite(w, r, r.URL.Query().Get("return"))
}

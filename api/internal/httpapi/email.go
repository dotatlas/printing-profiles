package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/dotatlas/printing-profiles/api/internal/db"
)

// maxEmailLength is the longest address that SMTP allows.
const maxEmailLength = 254

// readJSON decodes the JSON body into v, or sends a 400 problem and returns false. Write handlers call it first.
func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeProblem(w, http.StatusBadRequest, "the body must be a JSON object with the documented fields")
		return false
	}
	return true
}

// normalEmail returns the address trimmed and in lower case, or false when it is not one plain address.
// Nothing else changes (no rules for dots or plus signs), so two spellings stay two addresses.
func normalEmail(s string) (string, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	a, err := mail.ParseAddress(s)
	if err != nil || a.Address != s || a.Name != "" || len(s) > maxEmailLength || !strings.Contains(s[strings.LastIndex(s, "@")+1:], ".") {
		return "", false
	}
	return s, true
}

// handleEmailStart sends a one-time sign-in link to an address. It answers the same for every valid address,
// so nobody can learn which addresses have accounts. The router calls it for POST /api/v1/auth/email/start.
func (s *Server) handleEmailStart(w http.ResponseWriter, r *http.Request) {
	if s.Mail == nil {
		writeProblem(w, http.StatusServiceUnavailable, "email sign-in is not available; use GitHub or Discord")
		return
	}
	var in struct {
		Email  string `json:"email"`
		Return string `json:"return"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	email, ok := normalEmail(in.Email)
	if !ok {
		writeProblem(w, http.StatusBadRequest, "enter one email address, like name@example.com")
		return
	}
	if !s.allow(w, r, "email-start-ip", s.clientIP(r), 20, time.Hour) || !s.allow(w, r, "email-start-address", email, 5, time.Hour) {
		return
	}
	ctx := r.Context()
	if err := s.DB.DeleteOldEmailTokens(ctx); err != nil {
		s.fail(w, r, err)
		return
	}
	token, hash := newToken()
	if err := s.DB.CreateEmailToken(ctx, db.CreateEmailTokenParams{TokenHash: hash, Email: email, ReturnPath: safeReturnPath(in.Return)}); err != nil {
		s.fail(w, r, err)
		return
	}
	// The token is in the fragment, which browsers never send to a server, so no log on the way can keep it.
	link := s.Cfg.SiteOrigin + "/auth/email#token=" + token
	body := "Open this link to sign in to Printing Profiles. It works one time, for 15 minutes:\n\n" + link +
		"\n\nIf you did not ask to sign in, you can ignore this email. Nobody can sign in without the link."
	if err := s.Mail.Send(ctx, email, "Your sign-in link for Printing Profiles", body); err != nil {
		s.Log.Error("send sign-in email", "request_id", requestIDFrom(ctx), "err", err) // The address stays out of the log.
		writeProblem(w, http.StatusServiceUnavailable, "the email could not be sent; try again in a few minutes")
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "sent"})
}

// handleEmailFinish signs in with a token from an email link, one time only. The site's link page calls it with a
// button, because mail scanners open links on their own. The router calls it for POST /api/v1/auth/email/finish.
func (s *Server) handleEmailFinish(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Token string `json:"token"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	if !s.allow(w, r, "email-finish-ip", s.clientIP(r), 20, time.Hour) {
		return
	}
	ctx := r.Context()
	row, err := s.DB.UseEmailToken(ctx, hashToken(in.Token))
	if errors.Is(err, pgx.ErrNoRows) {
		writeProblem(w, http.StatusBadRequest, "this sign-in link does not work any more; ask for a new one")
		return
	}
	if err != nil {
		s.fail(w, r, err)
		return
	}
	userID, err := s.signIn(ctx, "email", row.Email, row.Email)
	if err == nil {
		err = s.startSession(w, r, userID)
	}
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"return": row.ReturnPath})
}

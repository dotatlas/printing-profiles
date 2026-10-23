package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/dotatlas/printing-profiles/api/internal/db"
)

// patchJSON sends a JSON PATCH from the site's origin with a session.
func patchJSON(s *Server, path, body, session string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodPatch, path, strings.NewReader(body))
	r.Header.Set("Origin", "http://localhost:3000")
	r.Header.Set("Content-Type", "application/json")
	r.AddCookie(&http.Cookie{Name: sessionCookie, Value: session})
	return serve(s, r)
}

// myIdentities returns the sign-in methods of the session's user.
func myIdentities(t *testing.T, s *Server, session string) []identityJSON {
	t.Helper()
	var list struct{ Items []identityJSON }
	rec := serve(s, withSession(http.MethodGet, "/api/v1/me/identities", session))
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("identities: status %d, body %s", rec.Code, rec.Body.String())
	}
	return list.Items
}

func TestOnboardingSetsAHandleOneTime(t *testing.T) {
	s, _ := newAuthServer(t)
	alice := signInAs(t, s, "alice")

	if rec := postJSON(s, "/api/v1/me/onboarding", `{"handle":"alice-prints","accept_terms":true}`, alice); rec.Code != http.StatusOK {
		t.Fatalf("onboarding: status %d, body %s", rec.Code, rec.Body.String())
	}
	var me meJSON
	_ = json.Unmarshal(serve(s, withSession(http.MethodGet, "/api/v1/me", alice)).Body.Bytes(), &me)
	if me.Handle != "alice-prints" || me.NeedsOnboarding {
		t.Errorf("me = %+v, want the handle and no onboarding", me)
	}
	if rec := postJSON(s, "/api/v1/me/onboarding", `{"handle":"another-one","accept_terms":true}`, alice); rec.Code != http.StatusConflict {
		t.Errorf("second onboarding: status %d, want 409", rec.Code)
	}
}

func TestOnboardingRefusesBadTakenAndReservedHandles(t *testing.T) {
	s, _ := newAuthServer(t)
	alice, bob := signInAs(t, s, "alice"), signInAs(t, s, "bob")
	postJSON(s, "/api/v1/me/onboarding", `{"handle":"printer-fan","accept_terms":true}`, alice)

	cases := map[string]int{
		`{"handle":"PRINTER-FAN","accept_terms":true}`: http.StatusBadRequest, // upper case is not a handle
		`{"handle":"printer-fan","accept_terms":true}`: http.StatusConflict,
		`{"handle":"admin","accept_terms":true}`:       http.StatusConflict,
		`{"handle":"ab","accept_terms":true}`:          http.StatusBadRequest,
		`{"handle":"a--b","accept_terms":true}`:        http.StatusBadRequest,
		`{"handle":"-ab","accept_terms":true}`:         http.StatusBadRequest,
		`{"handle":"bob-ok","accept_terms":false}`:     http.StatusBadRequest,
	}
	for body, want := range cases {
		if rec := postJSON(s, "/api/v1/me/onboarding", body, bob); rec.Code != want {
			t.Errorf("%s: status %d, want %d", body, rec.Code, want)
		}
	}
	if rec := postJSON(s, "/api/v1/me/onboarding", `{"handle":"bob-ok","accept_terms":true}`, ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("signed out: status %d, want 401", rec.Code)
	}
}

func TestDisplayNameChangesOnlyForTheSignedInUser(t *testing.T) {
	s, _ := newAuthServer(t)
	alice := signInAs(t, s, "alice")

	if rec := patchJSON(s, "/api/v1/me", `{"display_name":"  Alice P.  "}`, alice); rec.Code != http.StatusOK {
		t.Fatalf("status %d, body %s", rec.Code, rec.Body.String())
	}
	var me meJSON
	_ = json.Unmarshal(serve(s, withSession(http.MethodGet, "/api/v1/me", alice)).Body.Bytes(), &me)
	if me.DisplayName != "Alice P." {
		t.Errorf("display name = %q, want it trimmed", me.DisplayName)
	}
	for _, body := range []string{`{"display_name":"` + strings.Repeat("x", 61) + `"}`, `{"display_name":"a\u0007b"}`} {
		if rec := patchJSON(s, "/api/v1/me", body, alice); rec.Code != http.StatusBadRequest {
			t.Errorf("%.40s: status %d, want 400", body, rec.Code)
		}
	}
	if rec := patchJSON(s, "/api/v1/me", `{"display_name":"x"}`, "not-a-session"); rec.Code != http.StatusUnauthorized {
		t.Errorf("signed out: status %d, want 401", rec.Code)
	}
}

func TestTheLastSignInMethodCannotBeRemovedAndStrangersCannotRemoveAny(t *testing.T) {
	s, _ := newAuthServer(t)
	alice, mallory := signInAs(t, s, "alice"), signInAs(t, s, "mallory")
	only := myIdentities(t, s, alice)
	if len(only) != 1 || only[0].Provider != "fake" {
		t.Fatalf("alice's methods = %+v", only)
	}
	path := "/api/v1/me/identities/" + strconv.FormatInt(only[0].ID, 10)

	if rec := serve(s, withSession(http.MethodDelete, path, mallory)); rec.Code != http.StatusNotFound {
		t.Errorf("a stranger: status %d, want 404", rec.Code)
	}
	if rec := serve(s, withSession(http.MethodDelete, path, alice)); rec.Code != http.StatusConflict {
		t.Errorf("the last method: status %d, want 409", rec.Code)
	}

	aliceID, _ := s.signIn(context.Background(), "fake", "alice", "")
	if err := s.DB.AddIdentity(context.Background(), db.AddIdentityParams{UserID: aliceID, Provider: "github", ProviderUserID: "99"}); err != nil {
		t.Fatal(err)
	}
	if rec := serve(s, withSession(http.MethodDelete, path, alice)); rec.Code != http.StatusNoContent {
		t.Errorf("one of two methods: status %d, want 204", rec.Code)
	}
	if left := myIdentities(t, s, alice); len(left) != 1 || left[0].Provider != "github" {
		t.Errorf("alice's methods after the delete = %+v, want only github", left)
	}
}

func TestIdentitiesShowTheAddressOnlyForEmail(t *testing.T) {
	s, m := newEmailServer(t)
	postJSON(s, "/api/v1/auth/email/start", `{"email":"alice@example.com"}`, "")
	_, token := m.lastToken(t)
	rec := postJSON(s, "/api/v1/auth/email/finish", `{"token":"`+token+`"}`, "")
	session := rec.Result().Cookies()[0].Value

	items := myIdentities(t, s, session)
	if len(items) != 1 || items[0].Provider != "email" || items[0].Email != "alice@example.com" {
		t.Errorf("methods = %+v", items)
	}
	fake := myIdentities(t, s, signInAs(t, s, "bob"))
	if fake[0].Email != "" {
		t.Errorf("a provider method shows %q, want no email", fake[0].Email)
	}
}

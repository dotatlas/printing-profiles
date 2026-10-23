package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// reviewPage is the answer of the reviews list.
type reviewPage struct {
	Items      []reviewJSON `json:"items"`
	NextCursor string       `json:"next_cursor"`
}

// deleteJSON sends DELETE from the site's origin with a session.
func deleteJSON(s *Server, path, session string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(http.MethodDelete, path, nil)
	r.Header.Set("Origin", "http://localhost:3000")
	r.AddCookie(&http.Cookie{Name: sessionCookie, Value: session})
	return serve(s, r)
}

// ratingOf reads a preset's rating as the API gives it.
func ratingOf(t *testing.T, s *Server, id string) *ratingJSON {
	t.Helper()
	var p presetJSON
	if code := getJSON(s, "/api/v1/presets/"+id, "", &p); code != http.StatusOK {
		t.Fatalf("get preset: status %d", code)
	}
	return p.Rating
}

func TestReviewsNeedAMemberWhoIsNotTheAuthorOnAPublishedPreset(t *testing.T) {
	s, _ := newAuthServer(t)
	author := member(t, s, "reviewedauthor")
	p := createPreset(t, s, author, presetBody(`"status":"published"`))
	draft := createPreset(t, s, author, presetBody(`"status":"draft"`))
	reader := member(t, s, "reader")

	cases := []struct {
		name, path, body, token string
		want                    int
	}{
		{"a stranger", "/api/v1/presets/" + p.ID + "/reviews", `{"stars":5}`, "", http.StatusUnauthorized},
		{"no handle", "/api/v1/presets/" + p.ID + "/reviews", `{"stars":5}`, signInAs(t, s, "nohandlereviewer"), http.StatusForbidden},
		{"the author", "/api/v1/presets/" + p.ID + "/reviews", `{"stars":5}`, author, http.StatusForbidden},
		{"a draft", "/api/v1/presets/" + draft.ID + "/reviews", `{"stars":5}`, reader, http.StatusNotFound},
		{"no stars", "/api/v1/presets/" + p.ID + "/reviews", `{"body":"nice"}`, reader, http.StatusBadRequest},
		{"six stars", "/api/v1/presets/" + p.ID + "/reviews", `{"stars":6}`, reader, http.StatusBadRequest},
		{"half a star", "/api/v1/presets/" + p.ID + "/reviews", `{"stars":4.5}`, reader, http.StatusBadRequest},
		{"a long text", "/api/v1/presets/" + p.ID + "/reviews", `{"stars":4,"body":"` + strings.Repeat("a", 2001) + `"}`, reader, http.StatusBadRequest},
	}
	for _, c := range cases {
		if rec := postJSON(s, c.path, c.body, c.token); rec.Code != c.want {
			t.Errorf("%s: status %d, want %d (%s)", c.name, rec.Code, c.want, rec.Body)
		}
	}
	if r := ratingOf(t, s, p.ID); r != nil {
		t.Errorf("refused reviews changed the rating: %+v", r)
	}
}

func TestOneReviewPerMemberAndTotalsThatMatch(t *testing.T) {
	s, _ := newAuthServer(t)
	author := member(t, s, "ratedauthor")
	p := createPreset(t, s, author, presetBody(`"status":"published"`))
	first, second := member(t, s, "firstreviewer"), member(t, s, "secondreviewer")
	path := "/api/v1/presets/" + p.ID + "/reviews"

	if rec := postJSON(s, path, `{"stars":5,"body":"  Printed clean on my P1S.\nNo stringing.  "}`, first); rec.Code != http.StatusCreated {
		t.Fatalf("first review: status %d %s", rec.Code, rec.Body)
	}
	if r := ratingOf(t, s, p.ID); r == nil || r.Count != 1 || r.Average != 5 {
		t.Fatalf("after one review: %+v", r)
	}
	// A second POST replaces the member's review, and the totals follow.
	if rec := postJSON(s, path, `{"stars":2,"body":"Warped after all."}`, first); rec.Code != http.StatusOK {
		t.Fatalf("replace: status %d %s", rec.Code, rec.Body)
	}
	if rec := postJSON(s, path, `{"stars":3}`, second); rec.Code != http.StatusCreated {
		t.Fatalf("second member: status %d", rec.Code)
	}
	if r := ratingOf(t, s, p.ID); r == nil || r.Count != 2 || r.Average != 2.5 {
		t.Fatalf("after a replacement and a second review: %+v", r)
	}

	var page reviewPage
	getJSON(s, path, "", &page)
	if len(page.Items) != 2 || page.Items[0].Author.Handle != "secondreviewer" || page.Items[1].Body != "Warped after all." || !page.Items[1].Edited {
		t.Fatalf("list %+v", page.Items)
	}

	var mine reviewJSON
	if code := getJSON(s, path+"/mine", first, &mine); code != http.StatusOK || mine.Stars != 2 {
		t.Fatalf("my review: status %d %+v", code, mine)
	}
	if rec := deleteJSON(s, path+"/mine", first); rec.Code != http.StatusNoContent {
		t.Fatalf("delete mine: status %d", rec.Code)
	}
	if rec := deleteJSON(s, path+"/mine", first); rec.Code != http.StatusNotFound {
		t.Fatalf("delete again: status %d, want 404", rec.Code)
	}
	if r := ratingOf(t, s, p.ID); r == nil || r.Count != 1 || r.Average != 3 {
		t.Fatalf("after a delete: %+v", r)
	}
	if code := getJSON(s, path+"/mine", first, &mine); code != http.StatusNotFound {
		t.Fatalf("my review after delete: status %d, want 404", code)
	}
}

func TestReviewsComeInPagesNewestFirst(t *testing.T) {
	s, _ := newAuthServer(t)
	author := member(t, s, "pagedauthor")
	p := createPreset(t, s, author, presetBody(`"status":"published"`))
	for _, login := range []string{"pager-a", "pager-b", "pager-c"} {
		if rec := postJSON(s, "/api/v1/presets/"+p.ID+"/reviews", `{"stars":4}`, member(t, s, login)); rec.Code != http.StatusCreated {
			t.Fatalf("%s: status %d", login, rec.Code)
		}
	}
	var first, rest reviewPage
	getJSON(s, "/api/v1/presets/"+p.ID+"/reviews?limit=2", "", &first)
	if len(first.Items) != 2 || first.NextCursor == "" || first.Items[0].Author.Handle != "pager-c" {
		t.Fatalf("first page %+v", first)
	}
	getJSON(s, "/api/v1/presets/"+p.ID+"/reviews?limit=2&cursor="+first.NextCursor, "", &rest)
	if len(rest.Items) != 1 || rest.NextCursor != "" || rest.Items[0].Author.Handle != "pager-a" {
		t.Fatalf("second page %+v", rest)
	}
	if code := getJSON(s, "/api/v1/presets/"+p.ID+"/reviews?cursor=nope", "", &rest); code != http.StatusBadRequest {
		t.Fatalf("bad cursor: status %d", code)
	}
}

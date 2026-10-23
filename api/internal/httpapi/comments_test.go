package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// commentPage is the answer of the comments list.
type commentPage struct {
	Items      []commentJSON `json:"items"`
	NextCursor string        `json:"next_cursor"`
}

// comment posts a comment (a reply when parent is set) and gives the answer.
func comment(t *testing.T, s *Server, token, presetID, body, parent string) commentJSON {
	t.Helper()
	in := map[string]string{"body": body}
	if parent != "" {
		in["parent"] = parent
	}
	raw, _ := json.Marshal(in)
	rec := postJSON(s, "/api/v1/presets/"+presetID+"/comments", string(raw), token)
	var c commentJSON
	if rec.Code != http.StatusCreated || json.Unmarshal(rec.Body.Bytes(), &c) != nil {
		t.Fatalf("comment %q: status %d %s", body, rec.Code, rec.Body)
	}
	return c
}

func TestAStrangerCannotCommentOrChangeAComment(t *testing.T) {
	s, _ := newAuthServer(t)
	author := member(t, s, "commentpreset")
	p := createPreset(t, s, author, presetBody(`"status":"published"`))
	writer := member(t, s, "commentwriter")
	c := comment(t, s, writer, p.ID, "Does this work with the 0.6 nozzle?", "")
	other := member(t, s, "othercommenter")

	if rec := postJSON(s, "/api/v1/presets/"+p.ID+"/comments", `{"body":"hi"}`, ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("comment without a session: status %d, want 401", rec.Code)
	}
	if rec := postJSON(s, "/api/v1/presets/"+p.ID+"/comments", `{"body":"hi"}`, signInAs(t, s, "nohandlecomment")); rec.Code != http.StatusForbidden {
		t.Errorf("comment without a handle: status %d, want 403", rec.Code)
	}
	for _, token := range []string{"", other} {
		want := http.StatusNotFound
		if token == "" {
			want = http.StatusUnauthorized
		}
		if rec := patchJSON(s, "/api/v1/comments/"+c.ID, `{"body":"changed"}`, token); rec.Code != want {
			t.Errorf("edit by %q: status %d, want %d", token, rec.Code, want)
		}
		if rec := deleteJSON(s, "/api/v1/comments/"+c.ID, token); rec.Code != want {
			t.Errorf("delete by %q: status %d, want %d", token, rec.Code, want)
		}
	}
	var page commentPage
	getJSON(s, "/api/v1/presets/"+p.ID+"/comments", "", &page)
	if len(page.Items) != 1 || page.Items[0].Body != "Does this work with the 0.6 nozzle?" {
		t.Fatalf("the comment changed: %+v", page.Items)
	}
}

func TestCommentsHaveOneLevelOfRepliesOnAPublishedPreset(t *testing.T) {
	s, _ := newAuthServer(t)
	author := member(t, s, "threadauthor")
	p := createPreset(t, s, author, presetBody(`"status":"published"`))
	other := createPreset(t, s, author, presetBody(`"status":"published"`))
	draft := createPreset(t, s, author, presetBody(`"status":"draft"`))
	asker := member(t, s, "asker")

	question := comment(t, s, asker, p.ID, "  Bed temperature for the first layer?  ", "")
	answer := comment(t, s, author, p.ID, "65 °C, then 60.", question.ID)
	elsewhere := comment(t, s, asker, other.ID, "On another preset.", "")

	bad := map[string]string{
		"a reply to a reply":          `{"body":"x","parent":"` + answer.ID + `"}`,
		"a parent on another preset":  `{"body":"x","parent":"` + elsewhere.ID + `"}`,
		"a parent that is no comment": `{"body":"x","parent":"zzzzzzzzzzzz"}`,
		"an empty body":               `{"body":"   "}`,
		"a body over 2000 characters": `{"body":"` + strings.Repeat("a", 2001) + `"}`,
	}
	for name, body := range bad {
		if rec := postJSON(s, "/api/v1/presets/"+p.ID+"/comments", body, asker); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400 (%s)", name, rec.Code, rec.Body)
		}
	}
	if rec := postJSON(s, "/api/v1/presets/"+draft.ID+"/comments", `{"body":"x"}`, asker); rec.Code != http.StatusNotFound {
		t.Errorf("comment on a draft: status %d, want 404", rec.Code)
	}

	var page commentPage
	getJSON(s, "/api/v1/presets/"+p.ID+"/comments", "", &page)
	if len(page.Items) != 1 || page.Items[0].Body != "Bed temperature for the first layer?" || len(page.Items[0].Replies) != 1 ||
		page.Items[0].Replies[0].Author.Handle != "threadauthor" {
		t.Fatalf("thread %+v", page.Items)
	}
}

func TestEditAndDeleteKeepTheThread(t *testing.T) {
	s, _ := newAuthServer(t)
	author := member(t, s, "editthread")
	p := createPreset(t, s, author, presetBody(`"status":"published"`))
	writer := member(t, s, "editor")
	top := comment(t, s, writer, p.ID, "First question", "")
	reply := comment(t, s, author, p.ID, "An answer", top.ID)
	lonely := comment(t, s, writer, p.ID, "A second question", "")

	rec := patchJSON(s, "/api/v1/comments/"+top.ID, `{"body":"First question, edited"}`, writer)
	var edited commentJSON
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &edited) != nil || !edited.Edited || edited.Body != "First question, edited" {
		t.Fatalf("edit: status %d %s", rec.Code, rec.Body)
	}
	for _, id := range []string{top.ID, lonely.ID} {
		if rec := deleteJSON(s, "/api/v1/comments/"+id, writer); rec.Code != http.StatusNoContent {
			t.Fatalf("delete %s: status %d", id, rec.Code)
		}
	}
	if rec := deleteJSON(s, "/api/v1/comments/"+top.ID, writer); rec.Code != http.StatusNotFound {
		t.Fatalf("delete twice: status %d, want 404", rec.Code)
	}

	// The deleted comment with a reply stays as a placeholder without its text or author; the other one is gone.
	var page commentPage
	getJSON(s, "/api/v1/presets/"+p.ID+"/comments", "", &page)
	if len(page.Items) != 1 {
		t.Fatalf("items %+v", page.Items)
	}
	got := page.Items[0]
	if !got.Deleted || got.Body != "" || got.Author.Handle != "" || len(got.Replies) != 1 || got.Replies[0].ID != reply.ID {
		t.Fatalf("placeholder %+v", got)
	}
}

package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAStrangerHasNoBookmarks(t *testing.T) {
	s, _ := newAuthServer(t)
	author := member(t, s, "bookmarkedauthor")
	p := createPreset(t, s, author, presetBody(`"status":"published"`))
	if rec := postJSON(s, "/api/v1/me/bookmarks", `{"preset":"`+p.ID+`"}`, ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("add without a session: status %d, want 401", rec.Code)
	}
	if rec := deleteJSON(s, "/api/v1/me/bookmarks/"+p.ID, ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("remove without a session: status %d, want 401", rec.Code)
	}
	if code := getJSON(s, "/api/v1/me/bookmarks", "", nil); code != http.StatusUnauthorized {
		t.Errorf("list without a session: status %d, want 401", code)
	}
}

func TestBookmarksKeepPublishedPresetsNewestFirst(t *testing.T) {
	s, _ := newAuthServer(t)
	author := member(t, s, "savedauthor")
	first := createPreset(t, s, author, presetBody(`"status":"published"`))
	second := createPreset(t, s, author, presetBody(`"status":"published"`))
	draft := createPreset(t, s, author, presetBody(`"status":"draft"`))
	saver := member(t, s, "saver")

	if rec := postJSON(s, "/api/v1/me/bookmarks", `{"preset":"`+draft.ID+`"}`, saver); rec.Code != http.StatusNotFound {
		t.Errorf("bookmark a draft: status %d, want 404", rec.Code)
	}
	for _, id := range []string{first.ID, second.ID, second.ID} { // the second add of the same preset changes nothing
		if rec := postJSON(s, "/api/v1/me/bookmarks", `{"preset":"`+id+`"}`, saver); rec.Code != http.StatusNoContent {
			t.Fatalf("add %s: status %d %s", id, rec.Code, rec.Body)
		}
	}
	var mine struct {
		Items []presetJSON `json:"items"`
	}
	getJSON(s, "/api/v1/me/bookmarks", saver, &mine)
	if len(mine.Items) != 2 || mine.Items[0].ID != second.ID || mine.Items[1].ID != first.ID {
		t.Fatalf("bookmarks %+v", mine.Items)
	}
	var one struct {
		Bookmarked bool `json:"bookmarked"`
	}
	if getJSON(s, "/api/v1/me/bookmarks/"+first.ID, saver, &one); !one.Bookmarked {
		t.Fatal("first is not marked as bookmarked")
	}

	// A preset that its author deletes drops out of the list.
	if rec := deleteJSON(s, "/api/v1/presets/"+second.ID, author); rec.Code != http.StatusNoContent {
		t.Fatalf("delete preset: status %d", rec.Code)
	}
	if rec := deleteJSON(s, "/api/v1/me/bookmarks/"+first.ID, saver); rec.Code != http.StatusNoContent {
		t.Fatalf("remove: status %d", rec.Code)
	}
	getJSON(s, "/api/v1/me/bookmarks", saver, &mine)
	if len(mine.Items) != 0 {
		t.Fatalf("after a removal and a deleted preset: %+v", mine.Items)
	}
	if getJSON(s, "/api/v1/me/bookmarks/"+first.ID, saver, &one); one.Bookmarked {
		t.Fatal("first is still marked as bookmarked")
	}
}

func TestAProfileShowsTheMembersPublicCountsAndPresets(t *testing.T) {
	s, _ := newAuthServer(t)
	author := member(t, s, "profiled")
	_ = patchJSON(s, "/api/v1/me", `{"display_name":"Pat Profile"}`, author)
	p := createPreset(t, s, author, presetBody(`"status":"published"`))
	createPreset(t, s, author, presetBody(`"status":"draft"`))
	other := member(t, s, "profilereviewer")
	createPreset(t, s, other, presetBody(`"status":"published"`))
	if rec := postJSON(s, "/api/v1/presets/"+p.ID+"/reviews", `{"stars":4}`, other); rec.Code != http.StatusCreated {
		t.Fatalf("review: status %d", rec.Code)
	}

	var prof profileJSON
	if code := getJSON(s, "/api/v1/users/Profiled", "", &prof); code != http.StatusOK {
		t.Fatalf("profile: status %d", code)
	}
	if prof.Handle != "profiled" || prof.DisplayName != "Pat Profile" || prof.PresetCount != 1 || prof.ReviewCount != 0 || prof.JoinedAt.IsZero() {
		t.Fatalf("profile %+v", prof)
	}
	var reviewer profileJSON
	getJSON(s, "/api/v1/users/profilereviewer", "", &reviewer)
	if reviewer.ReviewCount != 1 || reviewer.PresetCount != 1 {
		t.Fatalf("reviewer profile %+v", reviewer)
	}
	for _, path := range []string{"/api/v1/users/nobody-here", "/api/v1/users/" + strings.Repeat("a", 40)} {
		if rec := serve(s, httptest.NewRequest(http.MethodGet, path, nil)); rec.Code != http.StatusNotFound {
			t.Errorf("%s: status %d, want 404", path, rec.Code)
		}
	}

	var page struct {
		Items []presetJSON `json:"items"`
	}
	getJSON(s, "/api/v1/presets?author=profiled", "", &page)
	if len(page.Items) != 1 || page.Items[0].ID != p.ID {
		t.Fatalf("browse by author %+v", page.Items)
	}
}

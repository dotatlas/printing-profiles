package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
)

// member signs in as login and gives the user a handle, so the user may write.
func member(t *testing.T, s *Server, login string) string {
	t.Helper()
	token := signInAs(t, s, login)
	if rec := postJSON(s, "/api/v1/me/onboarding", `{"handle":"`+login+`","accept_terms":true}`, token); rec.Code != http.StatusOK {
		t.Fatalf("onboarding %s: status %d", login, rec.Code)
	}
	return token
}

// presetBody is a valid create body; extra fields replace its defaults.
func presetBody(extra string) string {
	base := `"title":"PLA Basic tuned for the P1S","printer":"bambu-lab-p1s","material":"bambu-lab-pla-basic","slicer":"orcaslicer","nozzle_mm":0.4`
	if extra != "" {
		base += "," + extra
	}
	return "{" + base + "}"
}

// createPreset makes a preset through the API and returns it.
func createPreset(t *testing.T, s *Server, token, body string) presetJSON {
	t.Helper()
	rec := postJSON(s, "/api/v1/presets", body, token)
	var p presetJSON
	if rec.Code != http.StatusCreated || json.Unmarshal(rec.Body.Bytes(), &p) != nil {
		t.Fatalf("create: status %d, body %s", rec.Code, rec.Body.String())
	}
	return p
}

// getJSON sends GET with an optional session and decodes the answer into v. It returns the status.
func getJSON(s *Server, path, session string, v any) int {
	r := httptest.NewRequest(http.MethodGet, path, nil)
	if session != "" {
		r.AddCookie(&http.Cookie{Name: sessionCookie, Value: session})
	}
	rec := serve(s, r)
	_ = json.Unmarshal(rec.Body.Bytes(), v)
	return rec.Code
}

func TestSlugifyMakesShortASCIISlugs(t *testing.T) {
	cases := map[string]string{
		"PLA Basic tuned for the P1S":     "pla-basic-tuned-for-the-p1s",
		"  Crème brûlée PETG — 0.6 mm!! ": "creme-brulee-petg-0-6-mm",
		"!!!":                             "preset",
		"Ölçü / 速度 test":                  "olcu-test",
		strings.Repeat("silk ", 20):       "silk-silk-silk-silk-silk-silk-silk-silk-silk-silk-silk-silk",
		strings.Repeat("filament", 9):     strings.Repeat("filament", 9)[:60],
	}
	for in, want := range cases {
		if got := slugify(in); got != want {
			t.Errorf("slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPublicIDsUseTheReadableAlphabet(t *testing.T) {
	re := regexp.MustCompile(`^[a-km-np-z2-9]{8}$`)
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		id := newPublicID()
		if !re.MatchString(id) || seen[id] {
			t.Fatalf("id %q is not 8 readable characters, or repeated", id)
		}
		seen[id] = true
	}
}

func TestCreatePresetNeedsAMemberWithAHandle(t *testing.T) {
	s, _ := newAuthServer(t)
	if rec := postJSON(s, "/api/v1/presets", presetBody(""), ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("signed out: status %d, want 401", rec.Code)
	}
	if rec := postJSON(s, "/api/v1/presets", presetBody(""), signInAs(t, s, "no-handle")); rec.Code != http.StatusForbidden {
		t.Errorf("no handle: status %d, want 403", rec.Code)
	}
	p := createPreset(t, s, member(t, s, "alice"), presetBody(""))
	if p.Status != "draft" || p.Slug != "pla-basic-tuned-for-the-p1s" || p.Author.Handle != "alice" || p.License != "CC-BY-4.0" ||
		p.Printer.Model != "P1S" || p.Material.Type != "PLA" || p.Slicer.Name != "OrcaSlicer" || p.NozzleMM != 0.4 {
		t.Errorf("preset = %+v", p)
	}
}

func TestCreatePresetNamesTheBadField(t *testing.T) {
	s, _ := newAuthServer(t)
	alice := member(t, s, "alice")
	cases := map[string]string{
		`{"title":"PLA","printer":"bambu-lab-p1s","material":"bambu-lab-pla-basic","slicer":"orcaslicer","nozzle_mm":0.4}`: "title",
		presetBody(`"printer":"no-such-printer"`):                               "printer",
		presetBody(`"nozzle_mm":0.45`):                                          "nozzle_mm",
		presetBody(`"license":"MIT"`):                                           "license",
		presetBody(`"status":"hidden"`):                                         "status",
		`{"title":"PLA Basic tuned","printer":"bambu-lab-p1s","nozzle_mm":0.4}`: "material",
	}
	for body, field := range cases {
		rec := postJSON(s, "/api/v1/presets", body, alice)
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), field) {
			t.Errorf("%s: status %d, body %s, want 400 naming %s", body, rec.Code, rec.Body.String(), field)
		}
	}
}

func TestADraftIsVisibleOnlyToItsAuthor(t *testing.T) {
	s, _ := newAuthServer(t)
	alice, bob := member(t, s, "alice"), member(t, s, "bob")
	draft := createPreset(t, s, alice, presetBody(""))
	path := "/api/v1/presets/" + draft.ID

	var got presetJSON
	if code := getJSON(s, path, alice, &got); code != http.StatusOK || got.Description == nil {
		t.Errorf("author: status %d", code)
	}
	if code := getJSON(s, path, bob, &got); code != http.StatusNotFound {
		t.Errorf("another member: status %d, want 404", code)
	}
	if code := getJSON(s, path, "", &got); code != http.StatusNotFound {
		t.Errorf("signed out: status %d, want 404", code)
	}
	published := createPreset(t, s, alice, presetBody(`"status":"published"`))
	if code := getJSON(s, "/api/v1/presets/"+published.ID, "", &got); code != http.StatusOK || got.PublishedAt == nil {
		t.Errorf("published, signed out: status %d, published_at %v", code, got.PublishedAt)
	}
}

func TestAStrangerCannotEditOrDeleteAPreset(t *testing.T) {
	s, _ := newAuthServer(t)
	alice, mallory := member(t, s, "alice"), member(t, s, "mallory")
	p := createPreset(t, s, alice, presetBody(`"status":"published"`))
	path := "/api/v1/presets/" + p.ID

	if rec := patchJSON(s, path, `{"title":"Mallory was here"}`, mallory); rec.Code != http.StatusNotFound {
		t.Errorf("stranger edit: status %d, want 404", rec.Code)
	}
	if rec := serve(s, withSession(http.MethodDelete, path, mallory)); rec.Code != http.StatusNotFound {
		t.Errorf("stranger delete: status %d, want 404", rec.Code)
	}
	var got presetJSON
	getJSON(s, path, "", &got)
	if got.Title != p.Title {
		t.Errorf("title = %q after the stranger's edit", got.Title)
	}
}

func TestTheAuthorEditsAndSoftDeletes(t *testing.T) {
	s, pool := newAuthServer(t)
	alice := member(t, s, "alice")
	p := createPreset(t, s, alice, presetBody(""))
	path := "/api/v1/presets/" + p.ID

	rec := patchJSON(s, path, `{"title":"PETG tuned for the P1S","material":"prusa-research-prusament-petg","status":"published","description":"Line 1\nLine 2"}`, alice)
	var got presetJSON
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &got) != nil {
		t.Fatalf("edit: status %d, body %s", rec.Code, rec.Body.String())
	}
	if got.Slug != "petg-tuned-for-the-p1s" || got.Material.Type != "PETG" || got.Status != "published" || got.PublishedAt == nil ||
		*got.Description != "Line 1\nLine 2" || got.NozzleMM != 0.4 {
		t.Errorf("after edit = %+v", got)
	}

	if rec := serve(s, withSession(http.MethodDelete, path, alice)); rec.Code != http.StatusNoContent {
		t.Fatalf("delete: status %d", rec.Code)
	}
	if code := getJSON(s, path, alice, &got); code != http.StatusNotFound {
		t.Errorf("deleted, author: status %d, want 404", code)
	}
	var kept int
	if err := pool.QueryRow(t.Context(), "SELECT count(*) FROM presets WHERE deleted_at IS NOT NULL").Scan(&kept); err != nil || kept != 1 {
		t.Errorf("the row must stay after a soft delete: %d rows, err %v", kept, err)
	}
}

func TestBrowseFiltersAndPagesNewestFirst(t *testing.T) {
	s, _ := newAuthServer(t)
	alice := member(t, s, "alice")
	var made []presetJSON
	for i, mat := range []string{"bambu-lab-pla-basic", "prusa-research-prusament-petg", "bambu-lab-pla-basic"} {
		made = append(made, createPreset(t, s, alice, presetBody(`"status":"published","material":"`+mat+`","title":"Preset number `+string(rune('A'+i))+`"`)))
	}
	createPreset(t, s, alice, presetBody(`"title":"A draft nobody sees"`))

	var page pageJSON[presetJSON]
	if code := getJSON(s, "/api/v1/presets?limit=2", "", &page); code != http.StatusOK || len(page.Items) != 2 || page.NextCursor == "" {
		t.Fatalf("first page: status %d, %d items, cursor %q", code, len(page.Items), page.NextCursor)
	}
	if page.Items[0].ID != made[2].ID || page.Items[1].ID != made[1].ID || page.Items[0].Description != nil {
		t.Errorf("first page order = %s, %s; want the newest first, without descriptions", page.Items[0].ID, page.Items[1].ID)
	}
	var next pageJSON[presetJSON]
	getJSON(s, "/api/v1/presets?limit=2&cursor="+url.QueryEscape(page.NextCursor), "", &next)
	if len(next.Items) != 1 || next.Items[0].ID != made[0].ID || next.NextCursor != "" {
		t.Errorf("second page = %+v", next)
	}

	var petg pageJSON[presetJSON]
	getJSON(s, "/api/v1/presets?type=PETG&printer=bambu-lab-p1s&nozzle=0.4&slicer=orcaslicer", "", &petg)
	if len(petg.Items) != 1 || petg.Items[0].ID != made[1].ID {
		t.Errorf("PETG filter = %d items", len(petg.Items))
	}
	var none pageJSON[presetJSON]
	getJSON(s, "/api/v1/presets?brand=prusa-research&type=PLA", "", &none)
	if len(none.Items) != 0 || none.Items == nil {
		t.Errorf("no match must give an empty list, got %+v", none.Items)
	}
	for _, bad := range []string{"?nozzle=0.45", "?limit=0", "?limit=51", "?cursor=not-a-cursor"} {
		if code := getJSON(s, "/api/v1/presets"+bad, "", &none); code != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400", bad, code)
		}
	}
}

func TestMyPresetsListsDraftsForTheirAuthorOnly(t *testing.T) {
	s, _ := newAuthServer(t)
	alice, bob := member(t, s, "alice"), member(t, s, "bob")
	createPreset(t, s, alice, presetBody(""))
	createPreset(t, s, alice, presetBody(`"status":"published"`))

	var mine struct{ Items []presetJSON }
	getJSON(s, "/api/v1/me/presets", alice, &mine)
	var bobs struct{ Items []presetJSON }
	getJSON(s, "/api/v1/me/presets", bob, &bobs)
	if len(mine.Items) != 2 || len(bobs.Items) != 0 {
		t.Errorf("alice has %d, bob has %d; want 2 and 0", len(mine.Items), len(bobs.Items))
	}
	if code := getJSON(s, "/api/v1/me/presets", "", &bobs); code != http.StatusUnauthorized {
		t.Errorf("signed out: status %d, want 401", code)
	}
}

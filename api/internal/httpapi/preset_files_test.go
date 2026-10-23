package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// readyUpload uploads a fixture and completes it, and gives the upload ID.
func readyUpload(t *testing.T, s *Server, token, fixture string) string {
	t.Helper()
	data := presetFixture(t, fixture)
	ticket := startUpload(t, s, token, len(data))
	_ = s.Uploads.Put(t.Context(), "u/"+ticket.ID, data, uploadType, "")
	if rec := postJSON(s, "/api/v1/uploads/"+ticket.ID+"/complete", ``, token); rec.Code != http.StatusOK {
		t.Fatalf("complete %s: status %d %s", fixture, rec.Code, rec.Body)
	}
	return ticket.ID
}

func TestAPresetCarriesItsCheckedFileAndCountsDownloads(t *testing.T) {
	s, _, _ := newUploadServer(t)
	s.Cfg.APIOrigin = "https://api.test"
	token := member(t, s, "fileowner")
	file := readyUpload(t, s, token, "orca-filament.json")

	p := createPreset(t, s, token, presetBody(`"status":"published","file":"`+file+`"`))
	if p.File == nil || p.File.Slicer != "orcaslicer" || p.File.Kind != "filament" || len(p.File.Settings) != 7 ||
		p.File.DownloadURL != "https://api.test/api/v1/presets/"+p.ID+"/download" || !strings.HasPrefix(p.File.FileURL, "https://files.test/presets/") {
		t.Fatalf("file %+v", p.File)
	}

	rec := serve(s, httptest.NewRequest(http.MethodGet, "/api/v1/presets/"+p.ID+"/download", nil))
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != p.File.FileURL {
		t.Fatalf("download: status %d, Location %q", rec.Code, rec.Header().Get("Location"))
	}
	var again presetJSON
	getJSON(s, "/api/v1/presets/"+p.ID, "", &again)
	if again.DownloadCount != 1 {
		t.Fatalf("download count %d, want 1", again.DownloadCount)
	}
}

func TestAPresetTakesOnlyTheMembersOwnCheckedFileForItsSlicer(t *testing.T) {
	s, _, _ := newUploadServer(t)
	token := member(t, s, "careful")
	other := member(t, s, "otheruploader")
	theirs := readyUpload(t, s, other, "orca-filament.json")
	prusa := readyUpload(t, s, token, "prusa-filament.ini")
	pending := startUpload(t, s, token, 10).ID // never sent and never completed

	cases := map[string]string{
		"another member's file":     `"file":"` + theirs + `"`,
		"a file not checked yet":    `"file":"` + pending + `"`,
		"a file that is no ID":      `"file":"../etc"`,
		"a file for another slicer": `"file":"` + prusa + `"`, // the preset body's slicer is OrcaSlicer
	}
	for name, extra := range cases {
		rec := postJSON(s, "/api/v1/presets", presetBody(extra), token)
		if rec.Code != http.StatusBadRequest || !strings.HasPrefix(decodeProblem(t, rec).Detail, "file ") {
			t.Errorf("%s: status %d %s", name, rec.Code, rec.Body)
		}
	}

	// A PrusaSlicer file fits a PrusaSlicer preset, and an edit can add it later.
	p := createPreset(t, s, token, presetBody(`"slicer":"prusaslicer"`))
	rec := patchJSON(s, "/api/v1/presets/"+p.ID, `{"file":"`+prusa+`"}`, token)
	var got presetJSON
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &got) != nil || got.File == nil || got.File.Slicer != "prusaslicer" {
		t.Fatalf("edit with a file: status %d %s", rec.Code, rec.Body)
	}
	// Changing the slicer away from the file's slicer is refused too.
	if rec := patchJSON(s, "/api/v1/presets/"+p.ID, `{"slicer":"orcaslicer"}`, token); rec.Code != http.StatusBadRequest {
		t.Fatalf("slicer change against the file: status %d %s", rec.Code, rec.Body)
	}
}

func TestDownloadIsOnlyForAPublishedPresetWithAFile(t *testing.T) {
	s, _, _ := newUploadServer(t)
	token := member(t, s, "downloads")
	noFile := createPreset(t, s, token, presetBody(`"status":"published"`))
	draft := createPreset(t, s, token, presetBody(`"status":"draft","file":"`+readyUpload(t, s, token, "orca-process.json")+`"`))
	for name, id := range map[string]string{"no file": noFile.ID, "a draft": draft.ID, "a missing preset": "zzzzzzzz"} {
		if rec := serve(s, httptest.NewRequest(http.MethodGet, "/api/v1/presets/"+id+"/download", nil)); rec.Code != http.StatusNotFound {
			t.Errorf("%s: status %d, want 404", name, rec.Code)
		}
	}
}

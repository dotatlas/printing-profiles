package httpapi

import (
	"bytes"
	"encoding/json"
	"image"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

// testPhoto is a small JPEG of w×h pixels.
func testPhoto(t *testing.T, w, h int) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := jpeg.Encode(&b, image.NewRGBA(image.Rect(0, 0, w, h)), nil); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// uploadPhoto asks for a photo upload, puts data in the uploads bucket, and completes it. It gives the answer of
// "complete".
func uploadPhoto(t *testing.T, s *Server, token string, data []byte) (int, uploadJSON) {
	t.Helper()
	rec := postJSON(s, "/api/v1/uploads", `{"kind":"photo","size":`+strconv.Itoa(len(data))+`}`, token)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create photo upload: status %d %s", rec.Code, rec.Body)
	}
	var ticket uploadTicketJSON
	_ = json.Unmarshal(rec.Body.Bytes(), &ticket)
	_ = s.Uploads.Put(t.Context(), "u/"+ticket.ID, data, uploadType, "")
	rec = postJSON(s, "/api/v1/uploads/"+ticket.ID+"/complete", ``, token)
	var out uploadJSON
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	if out.ID == "" {
		out.ID = ticket.ID
	}
	return rec.Code, out
}

// readyPhoto uploads a small photo and gives its upload ID.
func readyPhoto(t *testing.T, s *Server, token string) string {
	t.Helper()
	code, up := uploadPhoto(t, s, token, testPhoto(t, 64, 32))
	if code != http.StatusOK {
		t.Fatalf("complete photo: status %d", code)
	}
	return up.ID
}

// attach adds an upload to a preset as a photo and gives the answer.
func attach(t *testing.T, s *Server, token, presetID, upload, alt string) (int, photoJSON) {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"upload": upload, "alt": alt})
	rec := postJSON(s, "/api/v1/presets/"+presetID+"/photos", string(body), token)
	var p photoJSON
	_ = json.Unmarshal(rec.Body.Bytes(), &p)
	return rec.Code, p
}

func TestAPhotoUploadIsCheckedAndReEncoded(t *testing.T) {
	s, _, files := newUploadServer(t)
	token := member(t, s, "photographer")

	code, up := uploadPhoto(t, s, token, testPhoto(t, 64, 32))
	if code != http.StatusOK || up.Status != "ready" || up.Width != 64 || up.Height != 32 {
		t.Fatalf("complete: status %d, upload %+v", code, up)
	}
	if !strings.HasPrefix(up.FileURL, "https://files.test/photos/") || !strings.HasSuffix(up.FileURL, ".jpg") ||
		up.ThumbURL != strings.TrimSuffix(up.FileURL, ".jpg")+"-thumb.jpg" {
		t.Fatalf("file %q, thumb %q", up.FileURL, up.ThumbURL)
	}
	for _, u := range []string{up.FileURL, up.ThumbURL} {
		o, found := files.Object(strings.TrimPrefix(u, "https://files.test/"))
		if !found || o.ContentType != "image/jpeg" || o.Disposition != "inline" {
			t.Fatalf("stored %s: %v %+v", u, found, o)
		}
	}

	if code, _ := uploadPhoto(t, s, token, []byte("<svg xmlns='http://www.w3.org/2000/svg'/>")); code != http.StatusUnprocessableEntity {
		t.Errorf("an SVG as a photo: status %d, want 422", code)
	}
	if rec := postJSON(s, "/api/v1/uploads", `{"kind":"photo","size":10485761}`, token); rec.Code != http.StatusBadRequest {
		t.Errorf("a photo over 10 MB: status %d, want 400", rec.Code)
	}
	if rec := postJSON(s, "/api/v1/uploads", `{"kind":"preset_file","size":2000000}`, token); rec.Code != http.StatusBadRequest {
		t.Errorf("a preset file of 2 MB: status %d, want 400", rec.Code)
	}
}

func TestAnAuthorAddsPhotosAndThePresetIsPhotoVerified(t *testing.T) {
	s, _, _ := newUploadServer(t)
	token := member(t, s, "verifier")
	p := createPreset(t, s, token, presetBody(`"status":"published"`))
	plain := createPreset(t, s, token, presetBody(`"status":"published","title":"No photo on this one"`))
	if p.Verified {
		t.Fatal("a preset without photos is verified")
	}

	code, photo := attach(t, s, token, p.ID, readyPhoto(t, s, token), "The benchy, front view")
	if code != http.StatusCreated || photo.Alt != "The benchy, front view" || photo.Width != 64 || !strings.HasPrefix(photo.URL, "https://files.test/photos/") {
		t.Fatalf("attach: status %d, photo %+v", code, photo)
	}

	var got presetJSON
	getJSON(s, "/api/v1/presets/"+p.ID, "", &got)
	if !got.Verified || len(got.Photos) != 1 || got.Photos[0].ID != photo.ID {
		t.Fatalf("preset after a photo: verified %v, photos %+v", got.Verified, got.Photos)
	}
	page := getPage[presetJSON](t, s, "/api/v1/presets?verified=true")
	if len(page.Items) != 1 || page.Items[0].ID != p.ID || !page.Items[0].Verified {
		t.Fatalf("verified only: %+v", page.Items)
	}
	if all := getPage[presetJSON](t, s, "/api/v1/presets"); len(all.Items) != 2 || all.Items[0].ID != plain.ID {
		t.Fatalf("all presets: %+v", all.Items)
	}
	if rec := serve(s, httptest.NewRequest(http.MethodGet, "/api/v1/presets?verified=yes", nil)); rec.Code != http.StatusBadRequest {
		t.Errorf("verified=yes: status %d, want 400", rec.Code)
	}

	if rec := deleteJSON(s, "/api/v1/photos/"+photo.ID, token); rec.Code != http.StatusNoContent {
		t.Fatalf("remove: status %d", rec.Code)
	}
	var after presetJSON
	getJSON(s, "/api/v1/presets/"+p.ID, "", &after)
	if after.Verified || len(after.Photos) != 0 {
		t.Fatalf("after the removal: verified %v, photos %+v", after.Verified, after.Photos)
	}
}

func TestAPhotoNeedsAltTextAReadyPhotoAndRoom(t *testing.T) {
	s, _, _ := newUploadServer(t)
	token := member(t, s, "carefulauthor")
	p := createPreset(t, s, token, presetBody(`"status":"published"`))
	up := readyPhoto(t, s, token)

	if code, _ := attach(t, s, token, p.ID, up, " "); code != http.StatusBadRequest {
		t.Errorf("no alt text: status %d, want 400", code)
	}
	if code, _ := attach(t, s, token, p.ID, up, strings.Repeat("a", 201)); code != http.StatusBadRequest {
		t.Errorf("alt text of 201 characters: status %d, want 400", code)
	}
	if code, _ := attach(t, s, token, p.ID, readyUpload(t, s, token, "orca-filament.json"), "A file"); code != http.StatusBadRequest {
		t.Errorf("a preset file as a photo: status %d, want 400", code)
	}
	if code, _ := attach(t, s, token, p.ID, up, "First"); code != http.StatusCreated {
		t.Fatalf("first photo: status %d", code)
	}
	if code, _ := attach(t, s, token, p.ID, up, "Again"); code != http.StatusConflict {
		t.Errorf("the same upload twice: status %d, want 409", code)
	}
	for i := 2; i <= maxPhotos; i++ {
		if code, _ := attach(t, s, token, p.ID, readyPhoto(t, s, token), "Photo "+strconv.Itoa(i)); code != http.StatusCreated {
			t.Fatalf("photo %d: status %d", i, code)
		}
	}
	if code, _ := attach(t, s, token, p.ID, readyPhoto(t, s, token), "One too many"); code != http.StatusConflict {
		t.Errorf("photo %d: status %d, want 409", maxPhotos+1, code)
	}
}

func TestAStrangerCannotAddOrRemoveAPhoto(t *testing.T) {
	s, _, _ := newUploadServer(t)
	author := member(t, s, "photoauthor")
	p := createPreset(t, s, author, presetBody(`"status":"published"`))
	authorUpload := readyPhoto(t, s, author)

	if code, _ := attach(t, s, "", p.ID, authorUpload, "Sneaky"); code != http.StatusUnauthorized {
		t.Errorf("without a session: status %d, want 401", code)
	}
	stranger := member(t, s, "photostranger")
	if code, _ := attach(t, s, stranger, p.ID, readyPhoto(t, s, stranger), "Not mine"); code != http.StatusNotFound {
		t.Errorf("on another member's preset: status %d, want 404", code)
	}
	own := createPreset(t, s, stranger, presetBody(`"status":"published"`))
	if code, _ := attach(t, s, stranger, own.ID, authorUpload, "Taken"); code != http.StatusBadRequest {
		t.Errorf("another member's upload: status %d, want 400", code)
	}

	_, photo := attach(t, s, author, p.ID, authorUpload, "The real one")
	if rec := deleteJSON(s, "/api/v1/photos/"+photo.ID, ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("remove without a session: status %d, want 401", rec.Code)
	}
	if rec := deleteJSON(s, "/api/v1/photos/"+photo.ID, stranger); rec.Code != http.StatusNotFound {
		t.Errorf("remove another member's photo: status %d, want 404", rec.Code)
	}
	var got presetJSON
	getJSON(s, "/api/v1/presets/"+p.ID, "", &got)
	if len(got.Photos) != 1 {
		t.Fatalf("the photo is gone after a stranger's removal: %+v", got.Photos)
	}
}

// The photo check holds one slot, so a second photo waits instead of decoding at the same time.
func TestPhotosDecodeOneAtATime(t *testing.T) {
	s, _, _ := newUploadServer(t)
	_ = s.Handler() // makes the slot
	token := member(t, s, "queued")
	s.photoSlot <- struct{}{} // another photo holds the slot
	done := make(chan int)
	go func() {
		code, _ := uploadPhoto(t, s, token, testPhoto(t, 8, 8))
		done <- code
	}()
	select {
	case code := <-done:
		t.Fatalf("the photo did not wait for the slot: status %d", code)
	case <-time.After(300 * time.Millisecond):
	}
	<-s.photoSlot
	if code := <-done; code != http.StatusOK {
		t.Fatalf("after the slot came free: status %d", code)
	}
}

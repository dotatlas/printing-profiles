package httpapi

import (
	"bytes"
	"encoding/json"
	"image"
	"image/jpeg"
	"net/http"
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

package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/dotatlas/printing-profiles/api/internal/storage"
)

// newUploadServer makes a test Server with a database and two in-memory buckets.
func newUploadServer(t *testing.T) (*Server, *storage.Memory, *storage.Memory) {
	t.Helper()
	s, _ := newAuthServer(t)
	uploads := &storage.Memory{PutURL: "https://uploads.test/"}
	files := &storage.Memory{}
	s.Uploads, s.Files = uploads, files
	s.Cfg.R2.FilesOrigin = "https://files.test"
	return s, uploads, files
}

// startUpload asks for an upload of size bytes and gives the answer.
func startUpload(t *testing.T, s *Server, token string, size int) uploadTicketJSON {
	t.Helper()
	rec := postJSON(s, "/api/v1/uploads", `{"kind":"preset_file","size":`+strconv.Itoa(size)+`}`, token)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create upload: status %d %s", rec.Code, rec.Body)
	}
	var ticket uploadTicketJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &ticket); err != nil {
		t.Fatal(err)
	}
	return ticket
}

// presetFixture reads one fixture of the presetfile package.
func presetFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("../presetfile/testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestUploadsAnswer503UntilStorageIsSetUp(t *testing.T) {
	s, _ := newAuthServer(t)
	token := member(t, s, "nostore")
	if rec := postJSON(s, "/api/v1/uploads", `{"kind":"preset_file","size":10}`, token); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d, want 503", rec.Code)
	}
}

func TestAStrangerCannotCreateOrCompleteAnUpload(t *testing.T) {
	s, _, _ := newUploadServer(t)
	if rec := postJSON(s, "/api/v1/uploads", `{"kind":"preset_file","size":10}`, ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("create without a session: status %d, want 401", rec.Code)
	}
	if rec := postJSON(s, "/api/v1/uploads/abcdefghjkmnpqrs/complete", ``, ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("complete without a session: status %d, want 401", rec.Code)
	}
	noHandle := signInAs(t, s, "nohandle")
	if rec := postJSON(s, "/api/v1/uploads", `{"kind":"preset_file","size":10}`, noHandle); rec.Code != http.StatusForbidden {
		t.Errorf("create without a handle: status %d, want 403", rec.Code)
	}

	owner := member(t, s, "owner")
	ticket := startUpload(t, s, owner, 10)
	stranger := member(t, s, "stranger")
	if rec := postJSON(s, "/api/v1/uploads/"+ticket.ID+"/complete", ``, stranger); rec.Code != http.StatusNotFound {
		t.Errorf("complete another member's upload: status %d, want 404", rec.Code)
	}
}

func TestCreateUploadSignsTheSizeAndRefusesBadRequests(t *testing.T) {
	s, _, _ := newUploadServer(t)
	token := member(t, s, "sizes")
	for _, body := range []string{`{"kind":"preset_file","size":0}`, `{"kind":"preset_file","size":1048577}`, `{"kind":"video","size":10}`, `{"size":10}`} {
		if rec := postJSON(s, "/api/v1/uploads", body, token); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400", body, rec.Code)
		}
	}
	ticket := startUpload(t, s, token, 383)
	if !regexp.MustCompile(`^[a-z2-9]{16}$`).MatchString(ticket.ID) {
		t.Errorf("id %q", ticket.ID)
	}
	if ticket.Method != "PUT" || !strings.Contains(ticket.URL, "u%2F"+ticket.ID+"?size=383&type=application%2Foctet-stream") ||
		ticket.Headers["Content-Type"] != "application/octet-stream" {
		t.Errorf("ticket %+v", ticket)
	}
}

func TestCompleteChecksTheFileAndStoresACleanCopy(t *testing.T) {
	s, uploads, files := newUploadServer(t)
	token := member(t, s, "uploader")
	data := presetFixture(t, "prusa-printer.ini")
	ticket := startUpload(t, s, token, len(data))

	if rec := postJSON(s, "/api/v1/uploads/"+ticket.ID+"/complete", ``, token); rec.Code != http.StatusConflict {
		t.Fatalf("complete before the upload: status %d, want 409", rec.Code)
	}
	_ = uploads.Put(t.Context(), "u/"+ticket.ID, data, "application/octet-stream", "")
	rec := postJSON(s, "/api/v1/uploads/"+ticket.ID+"/complete", ``, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("complete: status %d %s", rec.Code, rec.Body)
	}
	var got uploadJSON
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if got.Status != "ready" || got.Slicer != "prusaslicer" || got.Kind != "printer" || got.SlicerVersion != "2.9.6" ||
		strings.Join(got.Removed, ",") != "print_host,printhost_password,printhost_user" || len(got.Settings) != 3 {
		t.Fatalf("result %+v", got)
	}

	// The clean copy is in the files bucket under its content hash, as a download, without the secrets.
	key, ok := strings.CutPrefix(got.FileURL, "https://files.test/")
	if !ok || !regexp.MustCompile(`^presets/[0-9a-f]{64}\.ini$`).MatchString(key) {
		t.Fatalf("file url %q", got.FileURL)
	}
	o, ok := files.Object(key)
	if !ok || strings.Contains(string(o.Body), "hunter2") || o.ContentType != "text/plain; charset=utf-8" ||
		o.Disposition != `attachment; filename="prusaslicer-printer.ini"` {
		t.Fatalf("stored %v %q %q", ok, o.ContentType, o.Disposition)
	}

	// A second complete gives the same answer and writes nothing new.
	again := postJSON(s, "/api/v1/uploads/"+ticket.ID+"/complete", ``, token)
	if again.Code != http.StatusOK || again.Body.String() != rec.Body.String() {
		t.Fatalf("second complete: status %d %s", again.Code, again.Body)
	}
}

func TestCompleteRejectsAFileThatIsNotAPresetOrNotTheSignedSize(t *testing.T) {
	s, uploads, _ := newUploadServer(t)
	token := member(t, s, "rejects")

	html := []byte("<!doctype html><script>alert(1)</script>")
	ticket := startUpload(t, s, token, len(html))
	_ = uploads.Put(t.Context(), "u/"+ticket.ID, html, "application/octet-stream", "")
	rec := postJSON(s, "/api/v1/uploads/"+ticket.ID+"/complete", ``, token)
	if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(decodeProblem(t, rec).Detail, "not a preset") {
		t.Fatalf("html: status %d %s", rec.Code, rec.Body)
	}
	if again := postJSON(s, "/api/v1/uploads/"+ticket.ID+"/complete", ``, token); again.Code != http.StatusUnprocessableEntity {
		t.Fatalf("a rejected upload stays rejected: status %d", again.Code)
	}

	data := presetFixture(t, "superslicer-print.ini")
	small := startUpload(t, s, token, 10)
	_ = uploads.Put(t.Context(), "u/"+small.ID, data, "application/octet-stream", "")
	if rec := postJSON(s, "/api/v1/uploads/"+small.ID+"/complete", ``, token); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("a file larger than the signed size: status %d %s", rec.Code, rec.Body)
	}
}

// putFile sends a file the way a browser sends it to the dev upload URL of a ticket.
func putFile(s *Server, ticketURL string, body []byte, contentType string) *httptest.ResponseRecorder {
	u, _ := url.Parse(ticketURL)
	r := httptest.NewRequest(http.MethodPut, u.RequestURI(), bytes.NewReader(body))
	r.Header.Set("Origin", "http://localhost:3000")
	r.Header.Set("Content-Type", contentType)
	return serve(s, r)
}

func TestDevStoresUploadsInMemoryWithTheSignedSizeAndType(t *testing.T) {
	s, _, _ := newUploadServer(t)
	s.Uploads.(*storage.Memory).PutURL = "http://localhost:8080/api/v1/dev/uploads/"
	s.Cfg.R2.FilesOrigin = "http://localhost:8080/api/v1/dev/files"
	token := member(t, s, "devuploader")
	data := presetFixture(t, "orca-filament.json")
	ticket := startUpload(t, s, token, len(data))

	if rec := putFile(s, ticket.URL, data[:10], uploadType); rec.Code != http.StatusForbidden {
		t.Errorf("another size: status %d, want 403", rec.Code)
	}
	if rec := putFile(s, ticket.URL, data, "application/json"); rec.Code != http.StatusForbidden {
		t.Errorf("another type: status %d, want 403", rec.Code)
	}
	if rec := putFile(s, ticket.URL, data, uploadType); rec.Code != http.StatusOK {
		t.Fatalf("upload: status %d %s", rec.Code, rec.Body)
	}
	rec := postJSON(s, "/api/v1/uploads/"+ticket.ID+"/complete", ``, token)
	var got uploadJSON
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &got) != nil {
		t.Fatalf("complete: status %d %s", rec.Code, rec.Body)
	}

	// The dev files route serves the clean copy as a download that a browser never runs.
	u, _ := url.Parse(got.FileURL)
	file := serve(s, httptest.NewRequest(http.MethodGet, u.RequestURI(), nil))
	h := file.Header()
	if file.Code != http.StatusOK || h.Get("Content-Type") != "application/json" || h.Get("Content-Disposition") != `attachment; filename="orcaslicer-filament.json"` ||
		h.Get("X-Content-Type-Options") != "nosniff" || h.Get("Content-Security-Policy") != "sandbox" {
		t.Fatalf("file: status %d, headers %v", file.Code, h)
	}
}

func TestTheDevFileRoutesDoNotExistInProd(t *testing.T) {
	s, _, _ := newUploadServer(t)
	s.Cfg.Env = "prod"
	r := httptest.NewRequest(http.MethodPut, "/api/v1/dev/uploads/u%2Fabcdefghjkmnpqrs?size=1&type=application%2Foctet-stream", strings.NewReader("x"))
	r.Header.Set("Origin", "http://localhost:3000")
	if rec := serve(s, r); rec.Code != http.StatusNotFound {
		t.Errorf("dev upload in prod: status %d, want 404", rec.Code)
	}
	if rec := serve(s, httptest.NewRequest(http.MethodGet, "/api/v1/dev/files/presets/x.ini", nil)); rec.Code != http.StatusNotFound {
		t.Errorf("dev files in prod: status %d, want 404", rec.Code)
	}
}

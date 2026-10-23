package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/dotatlas/printing-profiles/api/internal/db"
	"github.com/dotatlas/printing-profiles/api/internal/presetfile"
	"github.com/dotatlas/printing-profiles/api/internal/storage"
)

// FileStore is one bucket: storage.Bucket for R2, storage.Memory in dev without R2 and in the tests.
type FileStore interface {
	PresignPut(key, contentType string, size int64, ttl time.Duration) string
	Get(ctx context.Context, key string, max int64) ([]byte, error)
	Put(ctx context.Context, key string, body []byte, contentType, disposition string) error
}

const (
	maxPresetFile = presetfile.MaxSize
	uploadTTL     = 10 * time.Minute
	// uploadType is the one type of every upload: the browser's file type is never trusted, and the check reads the content.
	uploadType = "application/octet-stream"
)

var uploadIDPattern = regexp.MustCompile(`^[a-z2-9]{16}$`)

// uploadTicketJSON is the answer of handleCreateUpload: where and how the browser sends the file.
type uploadTicketJSON struct {
	ID        string            `json:"id"`
	URL       string            `json:"url"`
	Method    string            `json:"method"`
	Headers   map[string]string `json:"headers"`
	ExpiresAt time.Time         `json:"expires_at"`
}

// uploadJSON is an upload after "complete": what the check found, or why it refused the file.
type uploadJSON struct {
	ID            string               `json:"id"`
	Status        string               `json:"status"`
	Slicer        string               `json:"slicer,omitempty"`
	Kind          string               `json:"kind,omitempty"`
	SlicerVersion string               `json:"slicer_version,omitempty"`
	Settings      []presetfile.Setting `json:"settings,omitempty"`
	Removed       []string             `json:"removed,omitempty"`
	FileURL       string               `json:"file_url,omitempty"`
}

// toUploadJSON maps an upload row to its answer. The file URL is on the files origin, never the API's.
func (s *Server) toUploadJSON(u db.Upload) uploadJSON {
	out := uploadJSON{ID: u.PublicID, Status: u.Status, Slicer: u.Slicer, Kind: u.FileKind, SlicerVersion: u.SlicerVersion, Removed: u.Removed}
	_ = json.Unmarshal(u.Settings, &out.Settings) // The API wrote the JSON itself, so it reads back.
	if u.FileKey != "" {
		out.FileURL = s.Cfg.R2.FilesOrigin + "/" + u.FileKey
	}
	return out
}

// storageReady sends 503 and returns false when the buckets are not set up. Both upload handlers call it.
func (s *Server) storageReady(w http.ResponseWriter) bool {
	if s.Uploads == nil || s.Files == nil {
		writeProblem(w, http.StatusServiceUnavailable, "file uploads are not set up yet")
		return false
	}
	return true
}

// handleCreateUpload makes an upload row and a presigned PUT for exactly the given size. The router calls it for
// POST /api/v1/uploads. The browser then sends the file straight to the bucket.
func (s *Server) handleCreateUpload(w http.ResponseWriter, r *http.Request) {
	u, ok := requireMember(w, r)
	if !ok || !s.storageReady(w) {
		return
	}
	var in struct {
		Kind string `json:"kind"`
		Size int64  `json:"size"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	switch {
	case in.Kind != "preset_file":
		writeProblem(w, http.StatusBadRequest, "kind must be preset_file")
		return
	case in.Size < 1 || in.Size > maxPresetFile:
		writeProblem(w, http.StatusBadRequest, "size must be from 1 to "+strconv.Itoa(maxPresetFile)+" bytes")
		return
	}
	if !s.allow(w, r, "upload-create", strconv.FormatInt(u.UserID, 10), 30, time.Hour) {
		return
	}
	id := randomID(16)
	if err := s.DB.CreateUpload(r.Context(), db.CreateUploadParams{PublicID: id, OwnerID: u.UserID, Kind: in.Kind, Size: int32(in.Size)}); err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, uploadTicketJSON{
		ID: id, URL: s.Uploads.PresignPut("u/"+id, uploadType, in.Size, uploadTTL), Method: http.MethodPut,
		Headers: map[string]string{"Content-Type": uploadType}, ExpiresAt: time.Now().Add(uploadTTL).UTC(),
	})
}

// handleCompleteUpload reads the uploaded file, checks it, and stores a clean copy in the files bucket. The router
// calls it for POST /api/v1/uploads/{id}/complete. A second call gives the same answer.
func (s *Server) handleCompleteUpload(w http.ResponseWriter, r *http.Request) {
	u, ok := requireMember(w, r)
	if !ok || !s.storageReady(w) {
		return
	}
	ctx := r.Context()
	id := r.PathValue("id")
	if !uploadIDPattern.MatchString(id) {
		writeProblem(w, http.StatusNotFound, "upload not found")
		return
	}
	up, err := s.DB.GetUpload(ctx, db.GetUploadParams{PublicID: id, OwnerID: u.UserID})
	if errors.Is(err, pgx.ErrNoRows) {
		writeProblem(w, http.StatusNotFound, "upload not found")
		return
	}
	if err != nil {
		s.fail(w, r, err)
		return
	}
	switch up.Status {
	case "ready":
		writeJSON(w, http.StatusOK, s.toUploadJSON(up))
		return
	case "rejected":
		writeProblem(w, http.StatusUnprocessableEntity, up.Reason)
		return
	}

	data, err := s.Uploads.Get(ctx, "u/"+id, int64(up.Size))
	switch {
	case errors.Is(err, storage.ErrNotFound):
		writeProblem(w, http.StatusConflict, "the file is not uploaded yet")
		return
	case errors.Is(err, storage.ErrTooLarge):
		s.reject(w, r, up.ID, "the file is not the size that the upload was for")
		return
	case err != nil:
		s.fail(w, r, err)
		return
	case len(data) != int(up.Size):
		s.reject(w, r, up.ID, "the file is not the size that the upload was for")
		return
	}
	f, err := presetfile.Parse(data)
	if err != nil {
		s.reject(w, r, up.ID, err.Error())
		return
	}

	sum := sha256.Sum256(f.Clean)
	ext, contentType := ".ini", "text/plain; charset=utf-8"
	if f.Slicer == "orcaslicer" {
		ext, contentType = ".json", "application/json"
	}
	// The key is the content hash, so a stored file never changes and the files origin can cache it for a year.
	key := "presets/" + hex.EncodeToString(sum[:]) + ext
	disposition := `attachment; filename="` + f.Slicer + "-" + f.Kind + ext + `"`
	if err := s.Files.Put(ctx, key, f.Clean, contentType, disposition); err != nil {
		s.fail(w, r, err)
		return
	}
	settings, err := json.Marshal(f.Key)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	removed := f.Removed
	if removed == nil {
		removed = []string{} // The column is NOT NULL, and pgx sends a nil list as NULL.
	}
	if _, err := s.DB.MarkUploadReady(ctx, db.MarkUploadReadyParams{
		ID: up.ID, Sha256: hex.EncodeToString(sum[:]), FileKey: key, Slicer: f.Slicer, FileKind: f.Kind,
		SlicerVersion: f.Version, Settings: settings, Removed: removed,
	}); err != nil {
		s.fail(w, r, err)
		return
	}
	// Read the row again: a parallel call may have finished first, and both callers get the stored answer.
	if up, err = s.DB.GetUpload(ctx, db.GetUploadParams{PublicID: id, OwnerID: u.UserID}); err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, s.toUploadJSON(up))
}

// reject marks an upload as refused with the reason and sends 422. handleCompleteUpload calls it.
func (s *Server) reject(w http.ResponseWriter, r *http.Request, id int64, reason string) {
	if err := s.DB.MarkUploadRejected(r.Context(), db.MarkUploadRejectedParams{ID: id, Reason: reason}); err != nil {
		s.fail(w, r, err)
		return
	}
	writeProblem(w, http.StatusUnprocessableEntity, reason)
}

package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/dotatlas/printing-profiles/api/internal/db"
	"github.com/dotatlas/printing-profiles/api/internal/images"
)

const (
	maxPhotos = 10  // the most photos on one preset
	maxAlt    = 200 // the longest alt text, in characters
)

// photoJSON is one photo of a preset: the full image and the thumbnail on the files origin, the size, the alt text.
type photoJSON struct {
	ID       string `json:"id"`
	URL      string `json:"url"`
	ThumbURL string `json:"thumb_url"`
	Width    int32  `json:"width"`
	Height   int32  `json:"height"`
	Alt      string `json:"alt"`
}

// completePhoto checks a photo upload, stores the full image and the thumbnail, and marks the upload ready.
// handleCompleteUpload calls it for an upload of kind "photo". One photo decodes at a time (photoSlot).
func (s *Server) completePhoto(w http.ResponseWriter, r *http.Request, up db.Upload, data []byte) bool {
	ctx := r.Context()
	select {
	case s.photoSlot <- struct{}{}:
		defer func() { <-s.photoSlot }()
	case <-ctx.Done():
		return false
	}
	p, err := images.Process(data)
	if err != nil {
		s.reject(w, r, up.ID, err.Error())
		return false
	}
	sum := sha256.Sum256(p.Full)
	key := "photos/" + hex.EncodeToString(sum[:])
	// The key is the content hash, so a stored photo never changes and the files origin can cache it for a year.
	if err := s.putPhoto(ctx, key+".jpg", p.Full); err != nil {
		s.fail(w, r, err)
		return false
	}
	if err := s.putPhoto(ctx, key+"-thumb.jpg", p.Thumb); err != nil {
		s.fail(w, r, err)
		return false
	}
	if _, err := s.DB.MarkPhotoReady(ctx, db.MarkPhotoReadyParams{
		ID: up.ID, Sha256: hex.EncodeToString(sum[:]), FileKey: key + ".jpg", ThumbKey: key + "-thumb.jpg",
		Width: int32(p.Width), Height: int32(p.Height),
	}); err != nil {
		s.fail(w, r, err)
		return false
	}
	return true
}

// putPhoto stores one JPEG in the files bucket, shown in the page (inline). completePhoto calls it for both copies.
func (s *Server) putPhoto(ctx context.Context, key string, body []byte) error {
	return s.Files.Put(ctx, key, body, "image/jpeg", "inline")
}

// photoURL gives the address of a stored photo on the files origin.
func (s *Server) photoURL(key string) string {
	return s.Cfg.R2.FilesOrigin + "/" + key
}

// handleAddPhoto adds a checked photo to the member's own preset. The router calls it for
// POST /api/v1/presets/{id}/photos.
func (s *Server) handleAddPhoto(w http.ResponseWriter, r *http.Request) {
	u, ok := requireMember(w, r)
	if !ok {
		return
	}
	var in struct {
		Upload string `json:"upload"`
		Alt    string `json:"alt"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	in.Alt = strings.TrimSpace(in.Alt)
	if in.Alt == "" || utf8.RuneCountInString(in.Alt) > maxAlt || strings.ContainsFunc(in.Alt, unicode.IsControl) {
		writeProblem(w, http.StatusBadRequest, "alt must have 1 to "+strconv.Itoa(maxAlt)+" characters and no control characters")
		return
	}
	ctx := r.Context()
	preset, err := s.DB.AuthorPreset(ctx, db.AuthorPresetParams{PublicID: r.PathValue("id"), AuthorID: u.UserID})
	if errors.Is(err, pgx.ErrNoRows) {
		writeProblem(w, http.StatusNotFound, "preset not found")
		return
	}
	if err != nil {
		s.fail(w, r, err)
		return
	}
	upload, err := s.DB.ReadyPhoto(ctx, db.ReadyPhotoParams{PublicID: in.Upload, OwnerID: u.UserID})
	if errors.Is(err, pgx.ErrNoRows) {
		writeProblem(w, http.StatusBadRequest, "upload must be one of your checked photos")
		return
	}
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if !s.allow(w, r, "photo-add", strconv.FormatInt(u.UserID, 10), 60, time.Hour) {
		return
	}
	// shortcut: two adds at the same moment can pass the limit by one; lock the preset row if that ever matters.
	n, err := s.DB.CountPhotos(ctx, preset)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if n >= maxPhotos {
		writeProblem(w, http.StatusConflict, "a preset can have at most "+strconv.Itoa(maxPhotos)+" photos")
		return
	}
	id := randomID(12)
	err = s.DB.AddPhoto(ctx, db.AddPhotoParams{PublicID: id, PresetID: preset, UploadID: upload, Alt: in.Alt})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		writeProblem(w, http.StatusConflict, "this photo is already on a preset")
		return
	}
	if err != nil {
		s.fail(w, r, err)
		return
	}
	photos, err := s.photosOf(ctx, preset)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	for _, p := range photos {
		if p.ID == id {
			writeJSON(w, http.StatusCreated, p)
			return
		}
	}
	s.fail(w, r, errors.New("the added photo is not in the list"))
}

// handleDeletePhoto removes a photo from the member's own preset. The router calls it for DELETE /api/v1/photos/{id}.
func (s *Server) handleDeletePhoto(w http.ResponseWriter, r *http.Request) {
	u, ok := requireMember(w, r)
	if !ok {
		return
	}
	n, err := s.DB.DeletePhoto(r.Context(), db.DeletePhotoParams{PublicID: r.PathValue("id"), AuthorID: u.UserID})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if n == 0 {
		writeProblem(w, http.StatusNotFound, "photo not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// photosOf lists the photos of a preset, oldest first. handleGetPreset and handleAddPhoto use it.
func (s *Server) photosOf(ctx context.Context, presetID int64) ([]photoJSON, error) {
	rows, err := s.DB.ListPhotos(ctx, presetID)
	if err != nil {
		return nil, err
	}
	out := make([]photoJSON, 0, len(rows))
	for _, p := range rows {
		out = append(out, photoJSON{
			ID: p.PublicID, URL: s.photoURL(p.FileKey), ThumbURL: s.photoURL(p.ThumbKey), Width: p.Width, Height: p.Height, Alt: p.Alt,
		})
	}
	return out, nil
}

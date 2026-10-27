package httpapi

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"

	"github.com/dotatlas/printing-profiles/api/internal/db"
	"github.com/dotatlas/printing-profiles/api/internal/images"
)

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

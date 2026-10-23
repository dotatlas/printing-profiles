package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/dotatlas/printing-profiles/api/internal/db"
)

// handleAddBookmark saves a published preset for the signed-in member. The router calls it for POST
// /api/v1/me/bookmarks. A second add of the same preset changes nothing.
func (s *Server) handleAddBookmark(w http.ResponseWriter, r *http.Request) {
	u, ok := requireUser(w, r)
	if !ok {
		return
	}
	var in struct {
		Preset string `json:"preset"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	p, err := s.DB.PublishedPreset(r.Context(), in.Preset)
	if errors.Is(err, pgx.ErrNoRows) {
		writeProblem(w, http.StatusNotFound, "preset not found")
		return
	}
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if !s.allow(w, r, "bookmark-write", strconv.FormatInt(u.UserID, 10), 100, time.Hour) {
		return
	}
	if err := s.DB.AddBookmark(r.Context(), db.AddBookmarkParams{UserID: u.UserID, PresetID: p.ID}); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleRemoveBookmark takes a preset out of the signed-in member's bookmarks. The router calls it for DELETE
// /api/v1/me/bookmarks/{preset}; a preset that was not bookmarked changes nothing.
func (s *Server) handleRemoveBookmark(w http.ResponseWriter, r *http.Request) {
	u, ok := requireUser(w, r)
	if !ok {
		return
	}
	if err := s.DB.RemoveBookmark(r.Context(), db.RemoveBookmarkParams{UserID: u.UserID, PublicID: r.PathValue("preset")}); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleIsBookmarked says if the signed-in member bookmarked a preset, for the toggle on the preset page. The router
// calls it for GET /api/v1/me/bookmarks/{preset}.
func (s *Server) handleIsBookmarked(w http.ResponseWriter, r *http.Request) {
	u, ok := requireUser(w, r)
	if !ok {
		return
	}
	yes, err := s.DB.IsBookmarked(r.Context(), db.IsBookmarkedParams{UserID: u.UserID, PublicID: r.PathValue("preset")})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"bookmarked": yes})
}

// handleListBookmarks lists the signed-in member's bookmarked presets that are still published, newest bookmark
// first. The router calls it for GET /api/v1/me/bookmarks.
func (s *Server) handleListBookmarks(w http.ResponseWriter, r *http.Request) {
	u, ok := requireUser(w, r)
	if !ok {
		return
	}
	rows, err := s.DB.ListBookmarks(r.Context(), u.UserID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	items := make([]presetJSON, 0, len(rows))
	for _, p := range rows {
		items = append(items, s.toPresetJSON(p, false))
	}
	writeJSON(w, http.StatusOK, map[string][]presetJSON{"items": items})
}

// profileJSON is a member's public profile.
type profileJSON struct {
	Handle      string    `json:"handle"`
	DisplayName string    `json:"display_name"`
	JoinedAt    time.Time `json:"joined_at"`
	PresetCount int64     `json:"preset_count"`
	ReviewCount int64     `json:"review_count"`
}

// handleProfile gives a member's public profile: the handle, the name, the join date and the counts. The router
// calls it for GET /api/v1/users/{handle}; the member's presets come from the browse route with author=handle.
func (s *Server) handleProfile(w http.ResponseWriter, r *http.Request) {
	handle := r.PathValue("handle")
	if len(handle) > 30 {
		writeProblem(w, http.StatusNotFound, "user not found")
		return
	}
	p, err := s.DB.GetProfile(r.Context(), handle)
	if errors.Is(err, pgx.ErrNoRows) {
		writeProblem(w, http.StatusNotFound, "user not found")
		return
	}
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, profileJSON{Handle: p.Handle, DisplayName: p.DisplayName, JoinedAt: p.CreatedAt, PresetCount: p.PresetCount, ReviewCount: p.ReviewCount})
}

package httpapi

import (
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/dotatlas/printing-profiles/api/internal/db"
)

const maxReviewBody = 2000 // the longest review text, in characters

// ratingJSON is a preset's rating: the number of reviews and the average stars with one decimal.
type ratingJSON struct {
	Count   int32   `json:"count"`
	Average float64 `json:"average"`
}

// rating gives the rating of a preset's totals, or nil for a preset without reviews. toPresetJSON calls it.
func rating(count, sum int32) *ratingJSON {
	if count == 0 {
		return nil
	}
	return &ratingJSON{Count: count, Average: math.Round(float64(sum)/float64(count)*10) / 10}
}

// reviewJSON is one review in the answers of the review routes.
type reviewJSON struct {
	Stars     int16      `json:"stars"`
	Body      string     `json:"body"`
	Author    authorJSON `json:"author"`
	CreatedAt time.Time  `json:"created_at"`
	Edited    bool       `json:"edited"`
}

// publishedPreset finds the published preset of the path, or sends 404 and returns false. The review routes call it.
func (s *Server) publishedPreset(w http.ResponseWriter, r *http.Request) (db.PublishedPresetRow, bool) {
	p, err := s.DB.PublishedPreset(r.Context(), r.PathValue("id"))
	if errors.Is(err, pgx.ErrNoRows) {
		writeProblem(w, http.StatusNotFound, "preset not found")
		return p, false
	}
	if err != nil {
		s.fail(w, r, err)
		return p, false
	}
	return p, true
}

// handleSaveReview makes the member's review of a published preset, or replaces it: one per member per preset. The
// router calls it for POST /api/v1/presets/{id}/reviews. It answers 201 for a new review and 200 for a replacement.
func (s *Server) handleSaveReview(w http.ResponseWriter, r *http.Request) {
	u, ok := requireMember(w, r)
	if !ok {
		return
	}
	var in struct {
		Stars *int   `json:"stars"`
		Body  string `json:"body"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	body := strings.TrimSpace(in.Body)
	switch {
	case in.Stars == nil || *in.Stars < 1 || *in.Stars > 5:
		writeProblem(w, http.StatusBadRequest, "stars must be a whole number from 1 to 5")
		return
	case utf8.RuneCountInString(body) > maxReviewBody:
		writeProblem(w, http.StatusBadRequest, "body must have at most 2000 characters")
		return
	}
	p, ok := s.publishedPreset(w, r)
	if !ok {
		return
	}
	if p.AuthorID == u.UserID {
		writeProblem(w, http.StatusForbidden, "you cannot review your own preset")
		return
	}
	if !s.allow(w, r, "review-write", strconv.FormatInt(u.UserID, 10), 20, time.Hour) {
		return
	}
	created, err := s.DB.UpsertReview(r.Context(), db.UpsertReviewParams{PresetID: p.ID, AuthorID: u.UserID, Stars: int16(*in.Stars), Body: body})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	mine, err := s.DB.MyReview(r.Context(), db.MyReviewParams{PresetID: p.ID, AuthorID: u.UserID})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	writeJSON(w, status, reviewJSON{
		Stars: mine.Stars, Body: mine.Body, Author: authorJSON{Handle: u.Handle, DisplayName: u.DisplayName},
		CreatedAt: mine.CreatedAt, Edited: mine.UpdatedAt.After(mine.CreatedAt),
	})
}

// handleMyReview gives the signed-in member's review of a preset, so the site can fill in the review form. The
// router calls it for GET /api/v1/presets/{id}/reviews/mine.
func (s *Server) handleMyReview(w http.ResponseWriter, r *http.Request) {
	u, ok := requireUser(w, r)
	if !ok {
		return
	}
	p, ok := s.publishedPreset(w, r)
	if !ok {
		return
	}
	mine, err := s.DB.MyReview(r.Context(), db.MyReviewParams{PresetID: p.ID, AuthorID: u.UserID})
	if errors.Is(err, pgx.ErrNoRows) {
		writeProblem(w, http.StatusNotFound, "you have not reviewed this preset")
		return
	}
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, reviewJSON{
		Stars: mine.Stars, Body: mine.Body, Author: authorJSON{Handle: u.Handle, DisplayName: u.DisplayName},
		CreatedAt: mine.CreatedAt, Edited: mine.UpdatedAt.After(mine.CreatedAt),
	})
}

// handleDeleteMyReview removes the signed-in member's review; the trigger takes it out of the totals. The router
// calls it for DELETE /api/v1/presets/{id}/reviews/mine.
func (s *Server) handleDeleteMyReview(w http.ResponseWriter, r *http.Request) {
	u, ok := requireUser(w, r)
	if !ok {
		return
	}
	p, ok := s.publishedPreset(w, r)
	if !ok {
		return
	}
	n, err := s.DB.DeleteMyReview(r.Context(), db.DeleteMyReviewParams{PresetID: p.ID, AuthorID: u.UserID})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if n == 0 {
		writeProblem(w, http.StatusNotFound, "you have not reviewed this preset")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleListReviews lists a published preset's reviews, newest first, with the cursor of the next page. The router
// calls it for GET /api/v1/presets/{id}/reviews.
func (s *Server) handleListReviews(w http.ResponseWriter, r *http.Request) {
	p, ok := s.publishedPreset(w, r)
	if !ok {
		return
	}
	limit := 20
	q := r.URL.Query()
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 50 {
			writeProblem(w, http.StatusBadRequest, "limit must be a number from 1 to 50")
			return
		}
		limit = n
	}
	params := db.ListReviewsParams{PresetID: p.ID, MaxRows: int32(limit) + 1}
	if v := q.Get("cursor"); v != "" {
		at, id, ok := decodeCursor(v)
		if !ok {
			writeProblem(w, http.StatusBadRequest, "cursor is not valid")
			return
		}
		params.AfterCreated = pgtype.Timestamptz{Time: at, Valid: true}
		params.AfterID = pgtype.Int8{Int64: id, Valid: true}
	}
	rows, err := s.DB.ListReviews(r.Context(), params)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	page := struct {
		Items      []reviewJSON `json:"items"`
		NextCursor string       `json:"next_cursor,omitempty"`
	}{Items: []reviewJSON{}}
	if len(rows) > limit {
		rows = rows[:limit]
		page.NextCursor = encodeCursor(rows[limit-1].CreatedAt, rows[limit-1].ID)
	}
	for _, row := range rows {
		page.Items = append(page.Items, reviewJSON{
			Stars: row.Stars, Body: row.Body, Author: authorJSON{Handle: row.AuthorHandle, DisplayName: row.AuthorDisplayName},
			CreatedAt: row.CreatedAt, Edited: row.UpdatedAt.After(row.CreatedAt),
		})
	}
	writeJSON(w, http.StatusOK, page)
}

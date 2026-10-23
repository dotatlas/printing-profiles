package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/dotatlas/printing-profiles/api/internal/db"
)

const maxCommentBody = 2000 // the longest comment, in characters

// commentJSON is one comment in the answers of the comment routes. A deleted comment has no text and no author.
type commentJSON struct {
	ID        string        `json:"id"`
	Body      string        `json:"body"`
	Author    authorJSON    `json:"author"`
	CreatedAt time.Time     `json:"created_at"`
	Edited    bool          `json:"edited"`
	Deleted   bool          `json:"deleted"`
	Replies   []commentJSON `json:"replies,omitempty"`
}

// commentBody checks and trims a comment text, or sends 400 and returns false. Create and edit call it.
func commentBody(w http.ResponseWriter, raw string) (string, bool) {
	body := strings.TrimSpace(raw)
	if body == "" || utf8.RuneCountInString(body) > maxCommentBody {
		writeProblem(w, http.StatusBadRequest, "body must have 1 to 2000 characters")
		return "", false
	}
	return body, true
}

// sendComment reads a comment back and sends it with the status. Create and edit answer with it.
func (s *Server) sendComment(w http.ResponseWriter, r *http.Request, status int, id string) {
	c, err := s.DB.GetComment(r.Context(), id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, status, commentJSON{
		ID: c.PublicID, Body: c.Body, Author: authorJSON{Handle: c.AuthorHandle, DisplayName: c.AuthorDisplayName},
		CreatedAt: c.CreatedAt, Edited: c.UpdatedAt.After(c.CreatedAt),
	})
}

// handleCreateComment adds a comment to a published preset, or a reply under one of its top-level comments. The
// router calls it for POST /api/v1/presets/{id}/comments.
func (s *Server) handleCreateComment(w http.ResponseWriter, r *http.Request) {
	u, ok := requireMember(w, r)
	if !ok {
		return
	}
	var in struct {
		Body   string  `json:"body"`
		Parent *string `json:"parent"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	body, ok := commentBody(w, in.Body)
	if !ok {
		return
	}
	p, ok := s.publishedPreset(w, r)
	if !ok {
		return
	}
	params := db.CreateCommentParams{PublicID: randomID(12), PresetID: p.ID, AuthorID: u.UserID, Body: body}
	if in.Parent != nil {
		parent, err := s.DB.CommentParent(r.Context(), db.CommentParentParams{PublicID: *in.Parent, PresetID: p.ID})
		if errors.Is(err, pgx.ErrNoRows) {
			writeProblem(w, http.StatusBadRequest, "parent must be a top-level comment of this preset")
			return
		}
		if err != nil {
			s.fail(w, r, err)
			return
		}
		params.ParentID = pgtype.Int8{Int64: parent, Valid: true}
	}
	if !s.allow(w, r, "comment-write", strconv.FormatInt(u.UserID, 10), 30, time.Hour) {
		return
	}
	if err := s.DB.CreateComment(r.Context(), params); err != nil {
		s.fail(w, r, err)
		return
	}
	s.sendComment(w, r, http.StatusCreated, params.PublicID)
}

// handleUpdateComment changes the text of the member's own comment. The router calls it for PATCH
// /api/v1/comments/{id}; another member's comment looks missing.
func (s *Server) handleUpdateComment(w http.ResponseWriter, r *http.Request) {
	u, ok := requireUser(w, r)
	if !ok {
		return
	}
	var in struct {
		Body string `json:"body"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	body, ok := commentBody(w, in.Body)
	if !ok {
		return
	}
	n, err := s.DB.UpdateComment(r.Context(), db.UpdateCommentParams{Body: body, PublicID: r.PathValue("id"), AuthorID: u.UserID})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if n == 0 {
		writeProblem(w, http.StatusNotFound, "comment not found")
		return
	}
	s.sendComment(w, r, http.StatusOK, r.PathValue("id"))
}

// handleDeleteComment removes the text of the member's own comment and marks it deleted. The router calls it for
// DELETE /api/v1/comments/{id}.
func (s *Server) handleDeleteComment(w http.ResponseWriter, r *http.Request) {
	u, ok := requireUser(w, r)
	if !ok {
		return
	}
	n, err := s.DB.DeleteComment(r.Context(), db.DeleteCommentParams{PublicID: r.PathValue("id"), AuthorID: u.UserID})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if n == 0 {
		writeProblem(w, http.StatusNotFound, "comment not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleListComments lists a published preset's top-level comments, oldest first, each with its replies. The
// router calls it for GET /api/v1/presets/{id}/comments.
func (s *Server) handleListComments(w http.ResponseWriter, r *http.Request) {
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
	params := db.ListTopCommentsParams{PresetID: p.ID, MaxRows: int32(limit) + 1}
	if v := q.Get("cursor"); v != "" {
		at, id, ok := decodeCursor(v)
		if !ok {
			writeProblem(w, http.StatusBadRequest, "cursor is not valid")
			return
		}
		params.AfterCreated = pgtype.Timestamptz{Time: at, Valid: true}
		params.AfterID = pgtype.Int8{Int64: id, Valid: true}
	}
	rows, err := s.DB.ListTopComments(r.Context(), params)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	page := struct {
		Items      []commentJSON `json:"items"`
		NextCursor string        `json:"next_cursor,omitempty"`
	}{Items: []commentJSON{}}
	if len(rows) > limit {
		rows = rows[:limit]
		page.NextCursor = encodeCursor(rows[limit-1].CreatedAt, rows[limit-1].ID)
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	replies, err := s.DB.ListReplies(r.Context(), ids)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	byParent := map[int64][]commentJSON{}
	for _, rep := range replies {
		byParent[rep.ParentID.Int64] = append(byParent[rep.ParentID.Int64], commentJSON{
			ID: rep.PublicID, Body: rep.Body, Author: authorJSON{Handle: rep.AuthorHandle, DisplayName: rep.AuthorDisplayName},
			CreatedAt: rep.CreatedAt, Edited: rep.UpdatedAt.After(rep.CreatedAt),
		})
	}
	for _, row := range rows {
		c := commentJSON{ID: row.PublicID, CreatedAt: row.CreatedAt, Replies: byParent[row.ID], Deleted: row.DeletedAt.Valid}
		if !c.Deleted {
			c.Body, c.Edited = row.Body, row.UpdatedAt.After(row.CreatedAt)
			c.Author = authorJSON{Handle: row.AuthorHandle, DisplayName: row.AuthorDisplayName}
		}
		page.Items = append(page.Items, c)
	}
	writeJSON(w, http.StatusOK, page)
}

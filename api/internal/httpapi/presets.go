package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/text/unicode/norm"

	"github.com/dotatlas/printing-profiles/api/internal/db"
	"github.com/dotatlas/printing-profiles/api/internal/presetfile"
)

const (
	maxTitle         = 100
	minTitle         = 5
	maxDescription   = 4000
	maxSlicerVersion = 20
	maxSlug          = 60
	defaultPresets   = 20
	maxPresetsPage   = 50
)

// nozzles, licenses and statuses are the values that the presets table allows. The handlers check input against them.
var (
	nozzles  = map[float64]bool{0.2: true, 0.25: true, 0.3: true, 0.4: true, 0.5: true, 0.6: true, 0.8: true, 1.0: true, 1.2: true}
	licenses = map[string]bool{"CC-BY-4.0": true, "CC-BY-SA-4.0": true, "CC-BY-NC-4.0": true, "CC0-1.0": true}
	statuses = map[string]bool{"draft": true, "published": true}
)

// idAlphabet has 32 characters without the look-alikes 0, o, 1 and l, so a public ID is easy to read and type.
const idAlphabet = "abcdefghijkmnpqrstuvwxyz23456789"

// newPublicID returns 8 random characters from idAlphabet (40 bits). createPreset tries again when one is taken.
func newPublicID() string {
	return randomID(8)
}

// randomID gives n random characters of idAlphabet. newPublicID uses it for presets, and the uploads for their IDs.
func randomID(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b) // In Go 1.24 and newer, crypto/rand.Read does not return an error.
	for i := range b {
		b[i] = idAlphabet[b[i]%32] // 256 is a multiple of 32, so every character is equally likely.
	}
	return string(b)
}

// slugify makes a URL slug from a title: ASCII letters and digits without accents, joined by single dashes, at most
// 60 characters. A title with no letters or digits gives "preset". createPreset and updatePreset call it.
func slugify(title string) string {
	var b strings.Builder
	dash := false
	for _, r := range norm.NFKD.String(strings.ToLower(title)) {
		switch {
		case unicode.Is(unicode.Mn, r):
			continue // the accent of a letter that NFKD split off
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			if dash && b.Len() > 0 {
				b.WriteByte('-')
			}
			dash = false
			b.WriteRune(r)
		default:
			dash = true
		}
	}
	s := b.String()
	if len(s) > maxSlug {
		cut := s[:maxSlug]
		// When the cut falls inside a word, go back to the last word end, if one is near.
		if i := strings.LastIndexByte(cut, '-'); s[maxSlug] != '-' && i > maxSlug/2 {
			cut = cut[:i]
		}
		s = strings.TrimRight(cut, "-")
	}
	if s == "" {
		return "preset"
	}
	return s
}

// brandJSON, printerRef, materialRef and slicerRef describe the catalog entries of a preset.
type printerRef struct {
	Slug  string    `json:"slug"`
	Model string    `json:"model"`
	Brand brandJSON `json:"brand"`
}

type materialRef struct {
	Slug        string    `json:"slug"`
	ProductName string    `json:"product_name"`
	Type        string    `json:"type"`
	Brand       brandJSON `json:"brand"`
}

// authorJSON is the public part of a preset's author.
type authorJSON struct {
	Handle      string `json:"handle"`
	DisplayName string `json:"display_name"`
}

// presetJSON is a preset in an API answer. Lists leave out the description.
type presetJSON struct {
	ID            string      `json:"id"`
	Slug          string      `json:"slug"`
	Title         string      `json:"title"`
	Description   *string     `json:"description,omitempty"`
	Author        authorJSON  `json:"author"`
	Printer       printerRef  `json:"printer"`
	Material      materialRef `json:"material"`
	NozzleMM      float64     `json:"nozzle_mm"`
	Slicer        slicerJSON  `json:"slicer"`
	SlicerVersion string      `json:"slicer_version"`
	License       string      `json:"license"`
	Status        string      `json:"status"`
	CreatedAt     time.Time   `json:"created_at"`
	UpdatedAt     time.Time   `json:"updated_at"`
	PublishedAt   *time.Time  `json:"published_at,omitempty"`
	File          *fileJSON   `json:"file,omitempty"`
	DownloadCount int64       `json:"download_count"`
	Rating        *ratingJSON `json:"rating,omitempty"`
}

// fileJSON is the checked preset file of a preset: what the check found, and where to download it.
type fileJSON struct {
	Slicer        string               `json:"slicer"`
	Kind          string               `json:"kind"`
	SlicerVersion string               `json:"slicer_version"`
	Settings      []presetfile.Setting `json:"settings"`
	DownloadURL   string               `json:"download_url"` // the API route that counts the download
	FileURL       string               `json:"file_url"`     // the file itself on the files origin
}

// toPresetJSON maps a preset_view row to its JSON. withDescription is true for the one-preset answer.
func (s *Server) toPresetJSON(p db.PresetView, withDescription bool) presetJSON {
	out := presetJSON{
		ID: p.PublicID, Slug: p.Slug, Title: p.Title,
		Author:   authorJSON{Handle: p.AuthorHandle, DisplayName: p.AuthorDisplayName},
		Printer:  printerRef{Slug: p.PrinterSlug, Model: p.PrinterModel, Brand: brandJSON{Slug: p.PrinterBrandSlug, Name: p.PrinterBrandName}},
		Material: materialRef{Slug: p.MaterialSlug, ProductName: p.MaterialProductName, Type: p.MaterialType, Brand: brandJSON{Slug: p.MaterialBrandSlug, Name: p.MaterialBrandName}},
		NozzleMM: p.NozzleMm, Slicer: slicerJSON{Slug: p.SlicerSlug, Name: p.SlicerName},
		SlicerVersion: p.SlicerVersion, License: p.License, Status: p.Status,
		CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt, DownloadCount: p.DownloadCount, Rating: rating(p.RatingCount, p.RatingSum),
	}
	if p.FileKey != "" {
		out.File = &fileJSON{
			Slicer: p.FileSlicer, Kind: p.FileKind, SlicerVersion: p.FileSlicerVersion,
			DownloadURL: s.Cfg.APIOrigin + "/api/v1/presets/" + p.PublicID + "/download", FileURL: s.Cfg.R2.FilesOrigin + "/" + p.FileKey,
		}
		_ = json.Unmarshal(p.FileSettings, &out.File.Settings) // The API wrote the JSON itself, so it reads back.
	}
	if withDescription {
		out.Description = &p.Description
	}
	if p.PublishedAt.Valid {
		out.PublishedAt = &p.PublishedAt.Time
	}
	return out
}

// requireMember returns the signed-in user when that user has a handle, or sends 401 or 403. Every write to user
// content calls it first, so content always has a public name beside it.
func requireMember(w http.ResponseWriter, r *http.Request) (sessionUser, bool) {
	u, ok := requireUser(w, r)
	if ok && u.Handle == "" {
		writeProblem(w, http.StatusForbidden, "choose a handle first")
		return u, false
	}
	return u, ok
}

// presetInput is the body of create and edit. Every field is a pointer, so edit can tell a missing field from an empty one.
type presetInput struct {
	Title         *string  `json:"title"`
	Description   *string  `json:"description"`
	Printer       *string  `json:"printer"`
	Material      *string  `json:"material"`
	NozzleMM      *float64 `json:"nozzle_mm"`
	Slicer        *string  `json:"slicer"`
	SlicerVersion *string  `json:"slicer_version"`
	License       *string  `json:"license"`
	Status        *string  `json:"status"`
	File          *string  `json:"file"` // the ID of a checked upload
}

// checked is presetInput after validation: trimmed text and catalog IDs. check makes it.
type checked struct {
	title, slug, description, slicerVersion, license, status pgtype.Text
	printer, material, slicer, file                          pgtype.Int8
	nozzle                                                   pgtype.Float8
	fileSlicer                                               string // the slicer format of the file, when one is given
}

// fileSlicers lists the catalog slicers that can read each preset file format. checkFile uses it.
var fileSlicers = map[string][]string{
	"prusaslicer": {"prusaslicer", "superslicer"},
	"superslicer": {"superslicer", "prusaslicer"},
	"orcaslicer":  {"orcaslicer", "bambu-studio", "creality-print"},
}

// fileFits says if a file in the format fileSlicer suits a preset for the catalog slicer slug.
func fileFits(fileSlicer, slug string) bool {
	return slices.Contains(fileSlicers[fileSlicer], slug)
}

// check validates the given fields and looks up the catalog slugs. It returns a problem detail that names the field,
// or "" when the input is good. For create, need is true and every required field must be there.
func (s *Server) check(ctx context.Context, in presetInput, need bool, authorID int64) (checked, string, error) {
	var c checked
	if in.File != nil {
		f, err := s.DB.ReadyUpload(ctx, db.ReadyUploadParams{PublicID: *in.File, OwnerID: authorID})
		if errors.Is(err, pgx.ErrNoRows) {
			return c, "file is not one of your checked uploads", nil
		}
		if err != nil {
			return c, "", err
		}
		c.file, c.fileSlicer = pgtype.Int8{Int64: f.ID, Valid: true}, f.Slicer
	}
	missing := func(name string) string { return name + " is required" }
	if in.Title != nil {
		t := strings.TrimSpace(*in.Title)
		if n := utf8.RuneCountInString(t); n < minTitle || n > maxTitle {
			return c, "title must have 5 to 100 characters", nil
		}
		c.title = pgtype.Text{String: t, Valid: true}
		c.slug = pgtype.Text{String: slugify(t), Valid: true}
	} else if need {
		return c, missing("title"), nil
	}
	if in.Description != nil {
		d := strings.TrimSpace(*in.Description)
		if utf8.RuneCountInString(d) > maxDescription {
			return c, "description must have at most 4000 characters", nil
		}
		c.description = pgtype.Text{String: d, Valid: true}
	} else if need {
		c.description = pgtype.Text{String: "", Valid: true}
	}
	if in.NozzleMM != nil {
		if !nozzles[*in.NozzleMM] {
			return c, "nozzle_mm must be one of 0.2, 0.25, 0.3, 0.4, 0.5, 0.6, 0.8, 1.0, 1.2", nil
		}
		c.nozzle = pgtype.Float8{Float64: *in.NozzleMM, Valid: true}
	} else if need {
		return c, missing("nozzle_mm"), nil
	}
	if in.SlicerVersion != nil {
		v := strings.TrimSpace(*in.SlicerVersion)
		if utf8.RuneCountInString(v) > maxSlicerVersion {
			return c, "slicer_version must have at most 20 characters", nil
		}
		c.slicerVersion = pgtype.Text{String: v, Valid: true}
	} else if need {
		c.slicerVersion = pgtype.Text{String: "", Valid: true}
	}
	if in.License != nil {
		if !licenses[*in.License] {
			return c, "license must be one of CC-BY-4.0, CC-BY-SA-4.0, CC-BY-NC-4.0, CC0-1.0", nil
		}
		c.license = pgtype.Text{String: *in.License, Valid: true}
	} else if need {
		c.license = pgtype.Text{String: "CC-BY-4.0", Valid: true}
	}
	if in.Status != nil {
		if !statuses[*in.Status] {
			return c, "status must be draft or published", nil
		}
		c.status = pgtype.Text{String: *in.Status, Valid: true}
	} else if need {
		c.status = pgtype.Text{String: "draft", Valid: true}
	}
	lookups := []struct {
		name string
		slug *string
		into *pgtype.Int8
		find func(context.Context, string) (int64, error)
	}{
		{"printer", in.Printer, &c.printer, s.DB.PrinterID},
		{"material", in.Material, &c.material, s.DB.MaterialID},
		{"slicer", in.Slicer, &c.slicer, s.DB.SlicerID},
	}
	for _, l := range lookups {
		if l.slug == nil {
			if need {
				return c, missing(l.name), nil
			}
			continue
		}
		id, err := l.find(ctx, *l.slug)
		if errors.Is(err, pgx.ErrNoRows) {
			return c, l.name + " is not in the catalog", nil
		}
		if err != nil {
			return c, "", err
		}
		*l.into = pgtype.Int8{Int64: id, Valid: true}
	}
	return c, "", nil
}

// handleCreatePreset makes a preset for the signed-in member. The router calls it for POST /api/v1/presets.
func (s *Server) handleCreatePreset(w http.ResponseWriter, r *http.Request) {
	u, ok := requireMember(w, r)
	if !ok {
		return
	}
	var in presetInput
	if !readJSON(w, r, &in) || !s.allow(w, r, "preset-create", strconv.FormatInt(u.UserID, 10), 20, time.Hour) {
		return
	}
	ctx := r.Context()
	c, problem, err := s.check(ctx, in, true, u.UserID)
	if problem == "" && c.fileSlicer != "" && !fileFits(c.fileSlicer, *in.Slicer) {
		problem = "file is a " + c.fileSlicer + " preset; choose a slicer that reads it"
	}
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if problem != "" {
		writeProblem(w, http.StatusBadRequest, problem)
		return
	}
	params := db.CreatePresetParams{
		AuthorID: u.UserID, Title: c.title.String, Slug: c.slug.String, Description: c.description.String,
		PrinterID: c.printer.Int64, MaterialID: c.material.Int64, NozzleMm: c.nozzle.Float64, SlicerID: c.slicer.Int64,
		SlicerVersion: c.slicerVersion.String, License: c.license.String, Status: c.status.String, FileUploadID: c.file,
	}
	// Two random IDs in a row are almost never taken; three tries make a clash practically impossible.
	for try := 0; ; try++ {
		params.PublicID = newPublicID()
		err = s.DB.CreatePreset(ctx, params)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && try < 2 {
			continue
		}
		break
	}
	if err != nil {
		s.fail(w, r, err)
		return
	}
	p, err := s.DB.GetPreset(ctx, params.PublicID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, s.toPresetJSON(p, true))
}

// handleGetPreset sends one preset: a published one to anyone, a draft only to its author. The router calls it for
// GET /api/v1/presets/{id}.
func (s *Server) handleGetPreset(w http.ResponseWriter, r *http.Request) {
	p, err := s.DB.GetPreset(r.Context(), r.PathValue("id"))
	u, signedIn := userFrom(r.Context())
	if errors.Is(err, pgx.ErrNoRows) || err == nil && p.Status == "draft" && (!signedIn || u.UserID != p.AuthorID) {
		writeProblem(w, http.StatusNotFound, "preset not found")
		return
	}
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, s.toPresetJSON(p, true))
}

// encodeCursor writes the position after one row (its time and ID) as opaque text. The preset and review lists use
// it, and decodeCursor reads it back.
func encodeCursor(at time.Time, id int64) string {
	return base64.RawURLEncoding.EncodeToString([]byte(at.UTC().Format(time.RFC3339Nano) + "," + strconv.FormatInt(id, 10)))
}

// decodeCursor reads a cursor that encodeCursor made, or returns false for anything else.
func decodeCursor(s string) (time.Time, int64, bool) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return time.Time{}, 0, false
	}
	at, id, ok := strings.Cut(string(raw), ",")
	t, err1 := time.Parse(time.RFC3339Nano, at)
	n, err2 := strconv.ParseInt(id, 10, 64)
	return t, n, ok && err1 == nil && err2 == nil
}

// text gives a query parameter as a nullable text, or null when it is not set.
func text(v string) pgtype.Text {
	return pgtype.Text{String: v, Valid: v != ""}
}

// handleListPresets sends one page of published presets, newest first, with the filters of the query. The router
// calls it for GET /api/v1/presets.
func (s *Server) handleListPresets(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	params := db.ListPresetsParams{
		Printer: text(q.Get("printer")), Material: text(q.Get("material")), Type: text(q.Get("type")),
		Brand: text(q.Get("brand")), Slicer: text(q.Get("slicer")), MaxRows: defaultPresets + 1,
	}
	for _, v := range []string{q.Get("printer"), q.Get("material"), q.Get("type"), q.Get("brand"), q.Get("slicer"), q.Get("cursor")} {
		if len(v) > maxSearchLength {
			writeProblem(w, http.StatusBadRequest, "filters and cursor must be 100 characters or fewer")
			return
		}
	}
	if v := q.Get("nozzle"); v != "" {
		n, err := strconv.ParseFloat(v, 64)
		if err != nil || !nozzles[n] {
			writeProblem(w, http.StatusBadRequest, "nozzle must be one of 0.2, 0.25, 0.3, 0.4, 0.5, 0.6, 0.8, 1.0, 1.2")
			return
		}
		params.Nozzle = pgtype.Float8{Float64: n, Valid: true}
	}
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > maxPresetsPage {
			writeProblem(w, http.StatusBadRequest, "limit must be a number from 1 to 50")
			return
		}
		params.MaxRows = int32(n) + 1
	}
	if v := q.Get("cursor"); v != "" {
		at, id, ok := decodeCursor(v)
		if !ok {
			writeProblem(w, http.StatusBadRequest, "cursor is not valid")
			return
		}
		params.AfterCreated = pgtype.Timestamptz{Time: at, Valid: true}
		params.AfterID = pgtype.Int8{Int64: id, Valid: true}
	}
	rows, err := s.DB.ListPresets(r.Context(), params)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	// One row more than the page tells whether a next page exists, with no COUNT(*).
	page := pageJSON[presetJSON]{Items: make([]presetJSON, 0, len(rows))}
	if len(rows) == int(params.MaxRows) {
		rows = rows[:len(rows)-1]
		page.NextCursor = encodeCursor(rows[len(rows)-1].CreatedAt, rows[len(rows)-1].ID)
	}
	for _, p := range rows {
		page.Items = append(page.Items, s.toPresetJSON(p, false))
	}
	writeJSON(w, http.StatusOK, page)
}

// handleMyPresets lists the signed-in user's presets, drafts included. The router calls it for GET /api/v1/me/presets.
func (s *Server) handleMyPresets(w http.ResponseWriter, r *http.Request) {
	u, ok := requireUser(w, r)
	if !ok {
		return
	}
	rows, err := s.DB.ListMyPresets(r.Context(), u.UserID)
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

// handleUpdatePreset changes the given fields of the member's own preset. Another user's preset answers 404, like
// a missing one. The router calls it for PATCH /api/v1/presets/{id}.
func (s *Server) handleUpdatePreset(w http.ResponseWriter, r *http.Request) {
	u, ok := requireMember(w, r)
	if !ok {
		return
	}
	var in presetInput
	if !readJSON(w, r, &in) {
		return
	}
	ctx := r.Context()
	c, problem, err := s.check(ctx, in, false, u.UserID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if problem == "" && (in.File != nil || in.Slicer != nil) {
		problem, err = s.checkFileFits(ctx, r.PathValue("id"), u.UserID, in, c)
		if err != nil {
			s.fail(w, r, err)
			return
		}
	}
	if problem != "" {
		writeProblem(w, http.StatusBadRequest, problem)
		return
	}
	id, err := s.DB.UpdatePreset(ctx, db.UpdatePresetParams{
		Title: c.title, Slug: c.slug, Description: c.description, PrinterID: c.printer, MaterialID: c.material,
		NozzleMm: c.nozzle, SlicerID: c.slicer, SlicerVersion: c.slicerVersion, License: c.license, Status: c.status,
		FileUploadID: c.file, PublicID: r.PathValue("id"), AuthorID: u.UserID,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		writeProblem(w, http.StatusNotFound, "preset not found")
		return
	}
	if err != nil {
		s.fail(w, r, err)
		return
	}
	p, err := s.DB.GetPreset(ctx, id)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, s.toPresetJSON(p, true))
}

// handleDeletePreset deletes the member's own preset softly: the row stays, but nobody can read it. The router calls
// it for DELETE /api/v1/presets/{id}.
func (s *Server) handleDeletePreset(w http.ResponseWriter, r *http.Request) {
	u, ok := requireMember(w, r)
	if !ok {
		return
	}
	n, err := s.DB.DeletePreset(r.Context(), db.DeletePresetParams{PublicID: r.PathValue("id"), AuthorID: u.UserID})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if n == 0 {
		writeProblem(w, http.StatusNotFound, "preset not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// checkFileFits checks, for an edit, that the file and the slicer still suit each other when either one changes.
// It reads the preset for the value that the edit keeps. Another user's preset is skipped: the edit then answers 404.
func (s *Server) checkFileFits(ctx context.Context, id string, authorID int64, in presetInput, c checked) (string, error) {
	p, err := s.DB.GetPreset(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil || p.AuthorID != authorID {
		return "", err
	}
	fileSlicer, slug := c.fileSlicer, p.SlicerSlug
	if fileSlicer == "" {
		fileSlicer = p.FileSlicer
	}
	if in.Slicer != nil {
		slug = *in.Slicer
	}
	if fileSlicer != "" && !fileFits(fileSlicer, slug) {
		return "file is a " + fileSlicer + " preset; choose a slicer that reads it", nil
	}
	return "", nil
}

// handlePresetDownload counts a download of a published preset's file and sends the browser to the file. The
// router calls it for GET /api/v1/presets/{id}/download.
func (s *Server) handlePresetDownload(w http.ResponseWriter, r *http.Request) {
	p, err := s.DB.GetPreset(r.Context(), r.PathValue("id"))
	if errors.Is(err, pgx.ErrNoRows) || err == nil && (p.Status != "published" || p.FileKey == "") {
		writeProblem(w, http.StatusNotFound, "preset file not found")
		return
	}
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if err := s.DB.CountDownload(r.Context(), p.ID); err != nil {
		s.fail(w, r, err)
		return
	}
	http.Redirect(w, r, s.Cfg.R2.FilesOrigin+"/"+p.FileKey, http.StatusFound)
}

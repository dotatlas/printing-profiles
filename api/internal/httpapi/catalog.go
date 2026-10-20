package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/dotatlas/printing-profiles/api/internal/db"
)

const (
	defaultPageSize = 20  // the page size when the client does not set limit
	maxPageSize     = 100 // the largest page that a client can ask for
	maxSearchLength = 100 // the longest search text that the API accepts
)

// materialTypes holds the filament types that the materials table allows. handleMaterials checks the type filter with it.
var materialTypes = map[string]bool{"PLA": true, "PETG": true, "ABS": true, "ASA": true, "TPU": true, "PA": true, "PC": true}

// likeEscaper makes the LIKE wildcards in user text into plain characters. searchPatterns uses it.
var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// brandJSON is a brand inside a printer or material answer.
type brandJSON struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

// printerJSON is one printer in the answer of handlePrinters.
type printerJSON struct {
	Slug  string    `json:"slug"`
	Model string    `json:"model"`
	Brand brandJSON `json:"brand"`
}

// materialJSON is one filament in the answer of handleMaterials. The ListMaterials query makes the Colors JSON list.
type materialJSON struct {
	Slug        string          `json:"slug"`
	ProductName string          `json:"product_name"`
	Type        string          `json:"type"`
	DiameterMM  float64         `json:"diameter_mm"`
	Brand       brandJSON       `json:"brand"`
	Colors      json.RawMessage `json:"colors"`
}

// slicerJSON is one slicer in the answer of handleSlicers.
type slicerJSON struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

// pageJSON is one page of a list. NextCursor is empty, and not sent, on the last page.
type pageJSON[T any] struct {
	Items      []T    `json:"items"`
	NextCursor string `json:"next_cursor,omitempty"`
}

// listQuery holds the search, cursor and page size of a list request. readListQuery makes it.
type listQuery struct {
	patterns []string
	after    string
	limit    int32
}

// readListQuery reads q, cursor and limit from the URL, or sends a 400 problem and returns false. The catalog handlers call it first.
func readListQuery(w http.ResponseWriter, r *http.Request) (listQuery, bool) {
	v := r.URL.Query()
	lq := listQuery{after: v.Get("cursor"), limit: defaultPageSize}

	q := v.Get("q")
	if len(q) > maxSearchLength || len(lq.after) > maxSearchLength {
		writeProblem(w, http.StatusBadRequest, "q and cursor must be 100 characters or fewer")
		return lq, false
	}
	lq.patterns = searchPatterns(q)

	if s := v.Get("limit"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 || n > maxPageSize {
			writeProblem(w, http.StatusBadRequest, "limit must be a number from 1 to 100")
			return lq, false
		}
		lq.limit = int32(n)
	}
	return lq, true
}

// searchPatterns makes one ILIKE pattern for each word of the search text. The queries require every pattern to match.
func searchPatterns(q string) []string {
	words := strings.Fields(q)
	patterns := make([]string, 0, len(words))
	for _, word := range words {
		patterns = append(patterns, "%"+likeEscaper.Replace(word)+"%")
	}
	return patterns
}

// writePage sends rows as one page. The queries return one extra row when a next page exists; writePage removes it and sets the cursor.
func writePage[T any](w http.ResponseWriter, items []T, limit int32, slugOf func(T) string) {
	page := pageJSON[T]{Items: items}
	if len(items) > int(limit) {
		page.Items = items[:limit]
		page.NextCursor = slugOf(page.Items[limit-1])
	}
	writeJSON(w, http.StatusOK, page)
}

// handlePrinters sends one page of the printers that match the search. The router calls it for GET /api/v1/printers.
func (s *Server) handlePrinters(w http.ResponseWriter, r *http.Request) {
	lq, ok := readListQuery(w, r)
	if !ok {
		return
	}
	rows, err := s.DB.ListPrinters(r.Context(), db.ListPrintersParams{Patterns: lq.patterns, After: lq.after, MaxRows: lq.limit + 1})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	items := make([]printerJSON, 0, len(rows))
	for _, p := range rows {
		items = append(items, printerJSON{Slug: p.Slug, Model: p.Model, Brand: brandJSON{Slug: p.BrandSlug, Name: p.BrandName}})
	}
	writePage(w, items, lq.limit, func(p printerJSON) string { return p.Slug })
}

// handleMaterials sends one page of the filaments that match the search, type and color filters. The router calls it for GET /api/v1/materials.
func (s *Server) handleMaterials(w http.ResponseWriter, r *http.Request) {
	lq, ok := readListQuery(w, r)
	if !ok {
		return
	}
	kind := strings.ToUpper(r.URL.Query().Get("type"))
	if kind != "" && !materialTypes[kind] {
		writeProblem(w, http.StatusBadRequest, "type must be one of PLA, PETG, ABS, ASA, TPU, PA, PC")
		return
	}
	color := r.URL.Query().Get("color")
	if len(color) > maxSearchLength {
		writeProblem(w, http.StatusBadRequest, "color must be 100 characters or fewer")
		return
	}
	if color != "" {
		color = "%" + likeEscaper.Replace(color) + "%"
	}
	rows, err := s.DB.ListMaterials(r.Context(), db.ListMaterialsParams{
		Patterns: lq.patterns, Type: kind, Color: color, After: lq.after, MaxRows: lq.limit + 1})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	items := make([]materialJSON, 0, len(rows))
	for _, m := range rows {
		items = append(items, materialJSON{Slug: m.Slug, ProductName: m.ProductName, Type: m.Type,
			DiameterMM: m.DiameterMm, Brand: brandJSON{Slug: m.BrandSlug, Name: m.BrandName}, Colors: m.Colors})
	}
	writePage(w, items, lq.limit, func(m materialJSON) string { return m.Slug })
}

// handleSlicers sends one page of the slicers that match the search. The router calls it for GET /api/v1/slicers.
func (s *Server) handleSlicers(w http.ResponseWriter, r *http.Request) {
	lq, ok := readListQuery(w, r)
	if !ok {
		return
	}
	rows, err := s.DB.ListSlicers(r.Context(), db.ListSlicersParams{Patterns: lq.patterns, After: lq.after, MaxRows: lq.limit + 1})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	items := make([]slicerJSON, 0, len(rows))
	for _, sl := range rows {
		items = append(items, slicerJSON{Slug: sl.Slug, Name: sl.Name})
	}
	writePage(w, items, lq.limit, func(sl slicerJSON) string { return sl.Slug })
}

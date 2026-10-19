package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/dotatlas/printing-profiles/api/internal/db"
	"github.com/dotatlas/printing-profiles/api/internal/dbtest"
)

// newCatalogServer makes a test Server with a fresh, migrated and seeded database. The catalog tests use it.
func newCatalogServer(t *testing.T) *Server {
	s := newTestServer()
	s.DB = db.New(dbtest.New(t))
	return s
}

// getPage sends GET path and decodes the JSON page, and fails the test when the status is not 200.
func getPage[T any](t *testing.T, s *Server, path string) pageJSON[T] {
	t.Helper()
	rec := serve(s, httptest.NewRequest(http.MethodGet, path, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s: status %d, body %s", path, rec.Code, rec.Body.String())
	}
	var page pageJSON[T]
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatalf("GET %s: body is not a page: %v", path, err)
	}
	return page
}

func TestSearchPatternsEscapeWildcardsAndSplitWords(t *testing.T) {
	cases := map[string][]string{
		"":              {},
		"  bambu  p1s ": {"%bambu%", "%p1s%"},
		"100%":          {`%100\%%`},
		"a_b":           {`%a\_b%`},
		`back\slash`:    {`%back\\slash%`},
	}
	for q, want := range cases {
		if got := searchPatterns(q); !reflect.DeepEqual(got, want) {
			t.Errorf("searchPatterns(%q) = %q, want %q", q, got, want)
		}
	}
}

func TestPrinterSearchFindsEveryWordInAnyOrder(t *testing.T) {
	page := getPage[printerJSON](t, newCatalogServer(t), "/api/v1/printers?q="+url.QueryEscape("p1s BAMBU"))

	if len(page.Items) != 1 || page.Items[0].Slug != "bambu-lab-p1s" || page.Items[0].Brand.Name != "Bambu Lab" {
		t.Errorf("items = %+v, want only bambu-lab-p1s", page.Items)
	}
}

func TestSearchTreatsWildcardsAsPlainText(t *testing.T) {
	s := newCatalogServer(t)
	for _, q := range []string{"%", "_"} {
		rec := serve(s, httptest.NewRequest(http.MethodGet, "/api/v1/slicers?q="+url.QueryEscape(q), nil))
		if body := strings.TrimSpace(rec.Body.String()); body != `{"items":[]}` {
			t.Errorf("q=%q: body %s, want an empty items list", q, body)
		}
	}
}

func TestPagesDoNotOverlapAndTheLastHasNoCursor(t *testing.T) {
	s := newCatalogServer(t)

	first := getPage[slicerJSON](t, s, "/api/v1/slicers?limit=4")
	if len(first.Items) != 4 || first.NextCursor != first.Items[3].Slug {
		t.Fatalf("first page = %+v, want 4 items and the last slug as next_cursor", first)
	}
	second := getPage[slicerJSON](t, s, "/api/v1/slicers?limit=4&cursor="+url.QueryEscape(first.NextCursor))
	if len(second.Items) != 2 || second.NextCursor != "" {
		t.Fatalf("second page = %+v, want the last 2 slicers and no next_cursor", second)
	}

	seen := map[string]bool{}
	for _, sl := range append(first.Items, second.Items...) {
		if seen[sl.Slug] {
			t.Errorf("slicer %s is on two pages", sl.Slug)
		}
		seen[sl.Slug] = true
	}
}

func TestMaterialTypeFilterIgnoresCase(t *testing.T) {
	page := getPage[materialJSON](t, newCatalogServer(t), "/api/v1/materials?type=petg&limit=100")

	if len(page.Items) == 0 {
		t.Fatal("no PETG materials, want the seeded ones")
	}
	for _, m := range page.Items {
		if m.Type != "PETG" {
			t.Errorf("material %s has type %s, want PETG", m.Slug, m.Type)
		}
	}
}

func TestBadListParametersAreProblem400(t *testing.T) {
	s := newCatalogServer(t)
	paths := []string{
		"/api/v1/printers?limit=0",
		"/api/v1/printers?limit=101",
		"/api/v1/printers?limit=x",
		"/api/v1/slicers?q=" + strings.Repeat("a", maxSearchLength+1),
		"/api/v1/slicers?cursor=" + strings.Repeat("a", maxSearchLength+1),
		"/api/v1/materials?type=wood",
	}
	for _, path := range paths {
		rec := serve(s, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("GET %s: status %d, want 400", path, rec.Code)
			continue
		}
		if p := decodeProblem(t, rec); p.Detail == "" {
			t.Errorf("GET %s: a 400 must say what is wrong", path)
		}
	}
}

func TestDatabaseErrorIsProblem500WithoutDetails(t *testing.T) {
	pool := dbtest.New(t)
	s := newTestServer()
	s.DB = db.New(pool)
	pool.Close() // Every query now fails.

	rec := serve(s, httptest.NewRequest(http.MethodGet, "/api/v1/printers", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if p := decodeProblem(t, rec); p.Detail != "" {
		t.Errorf("a 500 must not show details, got %q", p.Detail)
	}
}

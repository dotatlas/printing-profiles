package db_test

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/dotatlas/printing-profiles/api/internal/db"
	"github.com/dotatlas/printing-profiles/api/internal/dbtest"
)

func TestListPrintersMatchesEveryWordInAnyOrderAndCase(t *testing.T) {
	q := db.New(dbtest.New(t))

	rows, err := q.ListPrinters(context.Background(), db.ListPrintersParams{
		Patterns: []string{"%P1S%", "%bambu%"}, After: "", MaxRows: 10,
	})

	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Slug != "bambu-lab-p1s" || rows[0].BrandName != "Bambu Lab" {
		t.Errorf("rows = %+v, want only bambu-lab-p1s", rows)
	}
}

func TestListMaterialsFiltersByType(t *testing.T) {
	q := db.New(dbtest.New(t))

	rows, err := q.ListMaterials(context.Background(), db.ListMaterialsParams{
		Patterns: []string{}, Type: "TPU", After: "", MaxRows: 100,
	})

	if err != nil {
		t.Fatal(err)
	}
	if len(rows) == 0 {
		t.Fatal("no TPU materials, want the seeded ones")
	}
	for _, r := range rows {
		if r.Type != "TPU" || r.DiameterMm != 1.75 {
			t.Errorf("row %+v, want type TPU and diameter 1.75", r)
		}
	}
}

func TestListMaterialsFiltersByColorAndListsTheColors(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `INSERT INTO material_colors (material_id, name, hex)
		SELECT id, 'Glow Green', '#7cff4f' FROM materials WHERE slug = 'overture-tpu'`); err != nil {
		t.Fatal(err)
	}

	rows, err := db.New(pool).ListMaterials(ctx, db.ListMaterialsParams{
		Patterns: []string{}, Color: "%glow%", After: "", MaxRows: 100,
	})

	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Slug != "overture-tpu" {
		t.Fatalf("rows = %+v, want only overture-tpu", rows)
	}
	var colors []struct{ Name, Hex string }
	if err := json.Unmarshal(rows[0].Colors, &colors); err != nil {
		t.Fatalf("colors %s are not JSON: %v", rows[0].Colors, err)
	}
	want := []struct{ Name, Hex string }{{"Black", "#000000"}, {"Glow Green", "#7cff4f"}, {"White", "#ffffff"}}
	if !reflect.DeepEqual(colors, want) {
		t.Errorf("colors = %+v, want %+v in name order", colors, want)
	}
}

func TestListSlicersPagesByKeyWithoutGapsOrRepeats(t *testing.T) {
	q := db.New(dbtest.New(t))
	ctx := context.Background()

	var seen []string
	after := ""
	for page := 0; page < 10; page++ {
		rows, err := q.ListSlicers(ctx, db.ListSlicersParams{Patterns: []string{}, After: after, MaxRows: 4})
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) == 0 {
			break
		}
		for _, r := range rows {
			seen = append(seen, r.Slug)
		}
		after = rows[len(rows)-1].Slug
	}

	if len(seen) != 6 {
		t.Fatalf("paged through %d slicers %v, want all 6", len(seen), seen)
	}
	for i := 1; i < len(seen); i++ {
		if seen[i] <= seen[i-1] {
			t.Errorf("slugs not in strict order at %d: %q then %q", i, seen[i-1], seen[i])
		}
	}
}

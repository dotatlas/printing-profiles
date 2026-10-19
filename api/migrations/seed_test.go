package migrations_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/dotatlas/printing-profiles/api/internal/dbtest"
	"github.com/dotatlas/printing-profiles/api/migrations"
)

// seedVersion is the number of the migration that adds the starter catalog.
const seedVersion = 2

func TestSeedAddsKnownPrintersMaterialsAndSlicers(t *testing.T) {
	pool := dbtest.New(t)
	ctx := context.Background()

	var brand, model string
	err := pool.QueryRow(ctx, `SELECT b.name, p.model FROM printers p JOIN brands b ON b.id = p.brand_id
		WHERE p.slug = 'bambu-lab-p1s'`).Scan(&brand, &model)
	if err != nil || brand != "Bambu Lab" || model != "P1S" {
		t.Errorf("bambu-lab-p1s = (%q, %q, %v), want (Bambu Lab, P1S)", brand, model, err)
	}

	var material, kind string
	var diameter float64
	err = pool.QueryRow(ctx, `SELECT product_name, type, diameter_mm::float8 FROM materials
		WHERE slug = 'prusa-research-prusament-petg'`).Scan(&material, &kind, &diameter)
	if err != nil || material != "Prusament PETG" || kind != "PETG" || diameter != 1.75 {
		t.Errorf("prusa-research-prusament-petg = (%q, %q, %v, %v)", material, kind, diameter, err)
	}

	var slicers int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM slicers`).Scan(&slicers); err != nil || slicers != 6 {
		t.Errorf("slicers = %d (%v), want 6", slicers, err)
	}
}

func TestSeedDownRemovesOnlyTheSeededRows(t *testing.T) {
	db, err := sql.Open("pgx", dbtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	p, err := migrations.NewProvider(db)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := p.Up(ctx); err != nil {
		t.Fatal(err)
	}
	// A brand that a user adds later must stay when the seed goes down.
	if _, err := db.ExecContext(ctx, `INSERT INTO brands (name, slug) VALUES ('Test Brand', 'test-brand')`); err != nil {
		t.Fatal(err)
	}

	if _, err := p.DownTo(ctx, seedVersion-1); err != nil {
		t.Fatalf("down: %v", err)
	}

	var brands, printers int
	if err := db.QueryRowContext(ctx, `SELECT (SELECT count(*) FROM brands), (SELECT count(*) FROM printers)`).
		Scan(&brands, &printers); err != nil {
		t.Fatal(err)
	}
	if brands != 1 || printers != 0 {
		t.Errorf("after down: %d brands and %d printers, want only the user's brand and no printers", brands, printers)
	}
}

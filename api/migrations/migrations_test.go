package migrations_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/dotatlas/printing-profiles/api/internal/dbtest"
	"github.com/dotatlas/printing-profiles/api/migrations"
)

func TestMigrationsGoUpDownAndUpAgain(t *testing.T) {
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
		t.Fatalf("up: %v", err)
	}
	if _, err := p.DownTo(ctx, 0); err != nil {
		t.Fatalf("down to zero: %v", err)
	}
	if _, err := p.Up(ctx); err != nil {
		t.Fatalf("up again: %v", err)
	}

	version, err := p.GetDBVersion(ctx)
	if err != nil {
		t.Fatal(err)
	}
	sources := p.ListSources()
	if newest := sources[len(sources)-1].Version; version != newest {
		t.Errorf("database version = %d, want %d (the newest migration)", version, newest)
	}
}

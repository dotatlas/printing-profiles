// Package dbtest makes a fresh PostgreSQL database for one test, and drops it after the test. Only tests use it.
package dbtest

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"net/url"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" driver for database/sql, which goose uses

	"github.com/dotatlas/printing-profiles/api/migrations"
)

// defaultAdminURL is the docker compose database. TEST_DATABASE_URL replaces it, for example in CI.
const defaultAdminURL = "postgres://printing:printing@localhost:5432/printing?sslmode=disable"

// URL makes a new, empty database for this test and returns its URL. The migration tests use it directly.
func URL(t testing.TB) string {
	t.Helper()
	if testing.Short() {
		t.Skip("integration test: it needs PostgreSQL (start it with: docker compose up -d)")
	}
	admin := os.Getenv("TEST_DATABASE_URL")
	if admin == "" {
		admin = defaultAdminURL
	}
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, admin)
	if err != nil {
		t.Fatalf("cannot reach the test PostgreSQL (start it with: docker compose up -d): %v", err)
	}
	defer conn.Close(ctx)

	b := make([]byte, 6)
	_, _ = rand.Read(b) // In Go 1.24 and newer, crypto/rand.Read does not return an error.
	// The name is random hex, never user input. Sanitize quotes it as a safe SQL identifier.
	name := pgx.Identifier{"test_" + hex.EncodeToString(b)}.Sanitize()
	if _, err := conn.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatalf("create test database: %v", err)
	}
	t.Cleanup(func() {
		c, err := pgx.Connect(context.Background(), admin)
		if err != nil {
			t.Errorf("drop test database: %v", err)
			return
		}
		defer c.Close(context.Background())
		// FORCE closes connections that a test left open.
		if _, err := c.Exec(context.Background(), "DROP DATABASE "+name+" WITH (FORCE)"); err != nil {
			t.Errorf("drop test database: %v", err)
		}
	})

	u, err := url.Parse(admin)
	if err != nil {
		t.Fatal(err)
	}
	u.Path = "/" + name[1:len(name)-1] // Remove the quotes that Sanitize added.
	return u.String()
}

// New makes a new database with all migrations and returns a pool for it. The query and handler tests use it.
func New(t testing.TB) *pgxpool.Pool {
	t.Helper()
	dbURL := URL(t)
	ctx := context.Background()

	db, err := sql.Open("pgx", dbURL)
	if err != nil {
		t.Fatal(err)
	}
	p, err := migrations.NewProvider(db)
	if err == nil {
		_, err = p.Up(ctx)
	}
	db.Close()
	if err != nil {
		t.Fatalf("migrate test database: %v", err)
	}

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

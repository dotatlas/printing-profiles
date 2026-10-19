// Command api is the Printing Profiles API. Run it with a mode: "api serve" starts the HTTP server, "api migrate" updates the database.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" driver for database/sql, which goose uses

	"github.com/dotatlas/printing-profiles/api/internal/config"
	"github.com/dotatlas/printing-profiles/api/internal/db"
	"github.com/dotatlas/printing-profiles/api/internal/httpapi"
	"github.com/dotatlas/printing-profiles/api/migrations"
)

// main runs the mode that the first argument names, and exits with 1 on an error or 2 on a bad mode.
func main() {
	// All logs are JSON lines on stdout, so CloudWatch and Docker can read them.
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	mode := ""
	if len(os.Args) >= 2 {
		mode = os.Args[1]
	}
	var err error
	switch mode {
	case "serve":
		err = serve(log)
	case "migrate":
		err = migrate(log)
	default:
		fmt.Fprintln(os.Stderr, "usage: api serve | api migrate")
		os.Exit(2)
	}
	if err != nil {
		log.Error("stopped with an error", "mode", mode, "err", err)
		os.Exit(1)
	}
	log.Info("stopped", "mode", mode)
}

// serve loads the settings, connects to the database, opens the port and runs the API until a stop signal comes. main calls it for "serve".
func serve(log *slog.Logger) error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return fmt.Errorf("bad settings: %w", err)
	}
	pool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("database: %w", err)
	}
	defer pool.Close()
	// Stop now when the database cannot be reached, not at the first request.
	if err := pool.Ping(context.Background()); err != nil {
		return fmt.Errorf("database: %w", err)
	}
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Port))
	if err != nil {
		return err
	}
	// Ctrl+C and SIGTERM (from Docker or Lambda) cancel ctx, and Serve then stops.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Info("listening", "port", cfg.Port, "env", cfg.Env, "site_origin", cfg.SiteOrigin)
	srv := &httpapi.Server{Log: log, Cfg: cfg, DB: db.New(pool)}
	return httpapi.Serve(ctx, ln, srv.Handler())
}

// migrate applies every new migration to the database in DATABASE_URL, then returns. main calls it for "migrate".
func migrate(log *slog.Logger) error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return fmt.Errorf("bad settings: %w", err)
	}
	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	p, err := migrations.NewProvider(db)
	if err != nil {
		return err
	}
	results, err := p.Up(context.Background())
	for _, r := range results {
		log.Info("migration applied", "version", r.Source.Version, "duration_ms", r.Duration.Milliseconds())
	}
	if err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	log.Info("database is up to date", "applied", len(results))
	return nil
}

// Command api is the Printing Profiles API. Run it with a mode: "api serve" starts the HTTP server.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/dotatlas/printing-profiles/api/internal/config"
	"github.com/dotatlas/printing-profiles/api/internal/httpapi"
)

// main runs the mode that the first argument names, and exits with 1 on an error or 2 on a bad mode.
func main() {
	// All logs are JSON lines on stdout, so CloudWatch and Docker can read them.
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	if len(os.Args) < 2 || os.Args[1] != "serve" {
		fmt.Fprintln(os.Stderr, "usage: api serve")
		os.Exit(2)
	}
	if err := serve(log); err != nil {
		log.Error("stopped with an error", "err", err)
		os.Exit(1)
	}
	log.Info("stopped")
}

// serve loads the settings, opens the port and runs the API until a stop signal comes. main calls it for "serve".
func serve(log *slog.Logger) error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return fmt.Errorf("bad settings: %w", err)
	}
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Port))
	if err != nil {
		return err
	}
	// Ctrl+C and SIGTERM (from Docker or Lambda) cancel ctx, and Serve then stops.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Info("listening", "port", cfg.Port, "env", cfg.Env, "site_origin", cfg.SiteOrigin)
	srv := &httpapi.Server{Log: log, Cfg: cfg}
	return httpapi.Serve(ctx, ln, srv.Handler())
}

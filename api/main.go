// Command api is the Printing Profiles API. Run it with a mode: "api serve" starts the HTTP server.
package main

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

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
}

// serve loads the settings, opens the port and runs the API. main calls it for "serve".
func serve(log *slog.Logger) error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return fmt.Errorf("bad settings: %w", err)
	}
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Port))
	if err != nil {
		return err
	}
	log.Info("listening", "port", cfg.Port, "env", cfg.Env, "site_origin", cfg.SiteOrigin)
	srv := &httpapi.Server{Log: log, Cfg: cfg}
	// ReadHeaderTimeout stops a slow client from keeping a connection open forever.
	httpSrv := &http.Server{Handler: srv.Handler(), ReadHeaderTimeout: 10 * time.Second}
	return httpSrv.Serve(ln)
}

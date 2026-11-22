// Command server runs the Kanso HTTP API and serves the web app.
//
//	server -db kanso.db [-addr 127.0.0.1:8080] [-web web/dist]
//	server -db demo.db -demo -addr :8080
//	server -dump-openapi > api/openapi.json
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"kanso/internal/api"
	"kanso/internal/data"
	"kanso/internal/webui"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "server:", err)
		os.Exit(1)
	}
}

func run() error {
	dbPath := flag.String("db", "", "SQLite database written by cmd/ingest (saved systems are written to it)")
	addr := flag.String("addr", "127.0.0.1:8080", "listen address (localhost only by default: saved systems are writable without auth)")
	webDir := flag.String("web", "", "serve the web app from this directory instead of the one embedded in the binary")
	demo := flag.Bool("demo", false, "public demo: open the database read-only, refuse saved-system writes and rate-limit per IP")
	rateLimit := flag.Int("rate-limit", api.DefaultRateLimit, "demo only: requests a minute per IP to backtest, scan, quickscan, report and bars")
	dump := flag.Bool("dump-openapi", false, "print the OpenAPI spec as JSON and exit")
	flag.Parse()

	if *dump {
		spec, err := api.OpenAPIJSON()
		if err != nil {
			return err
		}
		_, err = os.Stdout.Write(spec)
		return err
	}
	if *dbPath == "" {
		return errors.New("-db is required")
	}

	log := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	prov, err := data.Open(*dbPath, *demo)
	if err != nil {
		return err
	}
	defer prov.Close()
	log.Info("opened database", "file", *dbPath, "read_only", *demo)

	cfg := api.Config{Logger: log, Demo: *demo, RateLimit: *rateLimit, Static: webui.FS()}
	if *webDir != "" {
		st, err := os.Stat(*webDir)
		if err != nil || !st.IsDir() {
			return fmt.Errorf("-web %s: not a directory", *webDir)
		}
		cfg.Static = os.DirFS(*webDir)
	}
	if cfg.Static == nil {
		log.Warn("no web app embedded (build with make build); serving the API only")
	}
	if *demo {
		log.Info("demo mode", "rate_limit_per_min", *rateLimit)
	}

	srv := &http.Server{Addr: *addr, Handler: api.New(prov, api.StoreSystems{S: prov}, cfg), ReadHeaderTimeout: 10 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	log.Info("listening", "addr", *addr)
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}

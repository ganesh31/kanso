// Command server runs the Kanso HTTP API and serves the web app.
//
//	server -db kanso.db [-addr :8080] [-web web/dist]
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
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "server:", err)
		os.Exit(1)
	}
}

func run() error {
	dbPath := flag.String("db", "", "SQLite database written by cmd/ingest (saved systems are written to it)")
	addr := flag.String("addr", ":8080", "listen address")
	webDir := flag.String("web", "web/dist", "directory of the built web app (empty: none)")
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
	prov, err := data.Open(*dbPath, false)
	if err != nil {
		return err
	}
	defer prov.Close()
	log.Info("opened database", "file", *dbPath)

	cfg := api.Config{Logger: log}
	if *webDir != "" {
		if st, err := os.Stat(*webDir); err == nil && st.IsDir() {
			cfg.Static = os.DirFS(*webDir)
		} else {
			log.Warn("web app not found; serving the API only", "dir", *webDir)
		}
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

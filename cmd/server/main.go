// Command server runs the Kanso HTTP API and serves the web app.
//
//	server -csv bars.csv [-addr :8080] [-web web/dist]
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
	"kanso/internal/market"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "server:", err)
		os.Exit(1)
	}
}

func run() error {
	csvPath := flag.String("csv", "", "CSV of daily bars (symbol,date,open,high,low,close,volume)")
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
	if *csvPath == "" {
		return errors.New("-csv is required")
	}

	log := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	prov, n, err := loadCSV(*csvPath)
	if err != nil {
		return err
	}
	log.Info("loaded bars", "file", *csvPath, "symbols", n)

	cfg := api.Config{Logger: log}
	if *webDir != "" {
		if st, err := os.Stat(*webDir); err == nil && st.IsDir() {
			cfg.Static = os.DirFS(*webDir)
		} else {
			log.Warn("web app not found; serving the API only", "dir", *webDir)
		}
	}

	srv := &http.Server{Addr: *addr, Handler: api.New(prov, cfg), ReadHeaderTimeout: 10 * time.Second}
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

func loadCSV(path string) (*market.MemProvider, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()
	bars, err := market.ReadCSV(f)
	if err != nil {
		return nil, 0, fmt.Errorf("%s: %w", path, err)
	}
	return market.MemProviderFromBars(bars), len(bars), nil
}

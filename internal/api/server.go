// Package api is Kanso's HTTP JSON API: huma v2 operations on the standard
// ServeMux, the OpenAPI 3.1 spec and docs page, and the web app's static
// files with an SPA fallback.
package api

import (
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"

	"kanso/internal/market"
)

// Config configures the server.
type Config struct {
	// Static holds the built web app (index.html at its root). Nil serves
	// no web app.
	Static fs.FS
	// Logger logs one line per request; nil disables request logging.
	Logger *slog.Logger
	// MaxBars caps the bars one /api/bars response serves (spec §6.4);
	// 0 means DefaultMaxBars.
	MaxBars int
	// Demo runs the public demo (spec §6.4): saved-system writes return
	// 403, and the computation and bars endpoints are rate-limited per
	// client IP.
	Demo bool
	// RateLimit is the demo's per-IP limit in requests a minute across
	// backtest, scan, quickscan, report and bars; 0 means DefaultRateLimit.
	// It has no effect outside demo mode.
	RateLimit int
	// TrustedProxies is how many proxies in front of the server append to
	// X-Forwarded-For (1 behind Cloud Run). 0 ignores the header, which a
	// client can set to anything, and uses the connection's address.
	TrustedProxies int

	// now is the rate limiter's clock; nil means time.Now.
	now func() time.Time
}

// DefaultMaxBars is the /api/bars cap when Config.MaxBars is 0.
const DefaultMaxBars = 5000

func init() {
	// Kanso never sends null for a list, so the generated client needn't
	// handle it.
	huma.DefaultArrayNullable = false
	// huma answers request validation failures with 422; Kanso's contract
	// says 400 (spec §7.4, plan Task 12).
	base := huma.NewError
	huma.NewError = func(status int, msg string, errs ...error) huma.StatusError {
		if status == http.StatusUnprocessableEntity {
			status = http.StatusBadRequest
		}
		return base(status, msg, errs...)
	}
}

// New returns the HTTP handler serving the API, /openapi.json, /docs and
// the web app. Saved systems live in systems.
func New(prov market.Provider, systems SystemStore, cfg Config) http.Handler {
	mux := http.NewServeMux()
	api := newAPI(mux)
	maxBars := cfg.MaxBars
	if maxBars <= 0 {
		maxBars = DefaultMaxBars
	}
	if cfg.Demo {
		limit := cfg.RateLimit
		if limit <= 0 {
			limit = DefaultRateLimit
		}
		api.UseMiddleware(refuseSystemWrites(api), limitRate(api, newLimiter(limit, cfg.now), cfg.TrustedProxies))
	}
	register(api, &handlers{prov: prov, systems: systems, maxBars: maxBars})
	mux.Handle("/", spa(cfg.Static))
	if cfg.Logger == nil {
		return mux
	}
	return logRequests(cfg.Logger, mux, cfg.TrustedProxies)
}

func newAPI(mux *http.ServeMux) huma.API {
	cfg := huma.DefaultConfig("Kanso API", "0.1.0")
	cfg.Info.Description = "Multi-year breakout scanner, stock report and system builder for NSE equities."
	// No $schema fields or Link headers in responses: the generated
	// TypeScript client is the contract.
	cfg.CreateHooks = nil
	return humago.New(mux, cfg)
}

// OpenAPIJSON returns the API's OpenAPI 3.1 spec as indented JSON. It is
// what `cmd/server -dump-openapi` writes to api/openapi.json.
func OpenAPIJSON() ([]byte, error) {
	api := newAPI(http.NewServeMux())
	register(api, &handlers{})
	b, err := json.MarshalIndent(api.OpenAPI(), "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

// spa serves files from static, and index.html for any other path that is
// not under /api/ so client-side routes work on reload.
func spa(static fs.FS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			notFoundProblem(w, r)
			return
		}
		if static == nil {
			http.NotFound(w, r)
			return
		}
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name != "" {
			if st, err := fs.Stat(static, name); err == nil && !st.IsDir() {
				http.ServeFileFS(w, r, static, name)
				return
			} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
		}
		index, err := fs.ReadFile(static, "index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		_, _ = w.Write(index)
	})
}

// notFoundProblem answers an unknown API route with the same RFC 9457
// problem JSON that huma uses for every other API error.
func notFoundProblem(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(http.StatusNotFound)
	_ = json.NewEncoder(w).Encode(huma.ErrorModel{
		Title:  http.StatusText(http.StatusNotFound),
		Status: http.StatusNotFound,
		Detail: "no API route for " + r.Method + " " + r.URL.Path,
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func logRequests(log *slog.Logger, next http.Handler, proxies int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		log.Info("request", "method", r.Method, "path", r.URL.Path, "status", rec.status,
			"ip", clientIP(r.Header.Get("X-Forwarded-For"), r.RemoteAddr, proxies),
			"duration_ms", time.Since(start).Milliseconds())
	})
}

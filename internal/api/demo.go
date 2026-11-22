package api

import (
	"fmt"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/danielgtaylor/huma/v2"
)

// DefaultRateLimit is the demo's per-IP limit, in requests a minute, when
// Config.RateLimit is 0.
const DefaultRateLimit = 60

// systemWrites are the operations demo mode refuses (spec §6.4: the demo
// server stores nothing; visitors' systems live in their browser).
var systemWrites = map[string]bool{
	"create-system": true,
	"update-system": true,
	"delete-system": true,
}

// rateLimited are the operations that compute or serve bars; in demo mode
// they share one per-IP limit.
var rateLimited = map[string]bool{
	"backtest":     true,
	"scan":         true,
	"quick-scan":   true,
	"stock-report": true,
	"get-bars":     true,
}

// refuseSystemWrites answers every saved-system write with a 403 before
// the input is read, so demo mode never writes whatever the store does.
func refuseSystemWrites(api huma.API) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		if systemWrites[ctx.Operation().OperationID] {
			_ = huma.WriteErr(api, ctx, http.StatusForbidden,
				"this is a read-only demo: saved systems are kept in your browser")
			return
		}
		next(ctx)
	}
}

// limitRate answers a rate-limited operation with a 429 problem and a
// Retry-After header once the client IP is over its limit.
func limitRate(api huma.API, l *limiter) func(huma.Context, func(huma.Context)) {
	return func(ctx huma.Context, next func(huma.Context)) {
		if !rateLimited[ctx.Operation().OperationID] {
			next(ctx)
			return
		}
		if wait, ok := l.allow(clientIP(ctx.Header("X-Forwarded-For"), ctx.RemoteAddr())); !ok {
			secs := int(math.Ceil(wait.Seconds()))
			ctx.SetHeader("Retry-After", strconv.Itoa(secs))
			_ = huma.WriteErr(api, ctx, http.StatusTooManyRequests,
				fmt.Sprintf("too many requests: at most %d a minute; try again in %d s", l.limit, secs))
			return
		}
		next(ctx)
	}
}

// clientIP is the first X-Forwarded-For hop when there is one (the demo
// runs behind one proxy), else the connection's address without its port.
func clientIP(xff, remoteAddr string) string {
	if first, _, _ := strings.Cut(xff, ","); strings.TrimSpace(first) != "" {
		return strings.TrimSpace(first)
	}
	if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
		return host
	}
	return remoteAddr
}

// limiter allows each key limit requests per fixed one-minute window. It
// is safe for concurrent use and forgets keys whose window has ended.
type limiter struct {
	limit int
	now   func() time.Time

	mu        sync.Mutex
	windows   map[string]*window
	lastSweep time.Time
}

type window struct {
	start time.Time
	count int
}

const rateWindow = time.Minute

func newLimiter(limit int, now func() time.Time) *limiter {
	if now == nil {
		now = time.Now
	}
	return &limiter{limit: limit, now: now, windows: map[string]*window{}}
}

// allow counts one request for key. When key is over its limit it returns
// false and how long until its window ends.
func (l *limiter) allow(key string) (time.Duration, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	if now.Sub(l.lastSweep) >= rateWindow {
		for k, w := range l.windows {
			if now.Sub(w.start) >= rateWindow {
				delete(l.windows, k)
			}
		}
		l.lastSweep = now
	}
	w, ok := l.windows[key]
	if !ok || now.Sub(w.start) >= rateWindow {
		w = &window{start: now}
		l.windows[key] = w
	}
	if w.count >= l.limit {
		return w.start.Add(rateWindow).Sub(now), false
	}
	w.count++
	return 0, true
}

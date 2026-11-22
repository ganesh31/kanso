// Command ingest backfills a stock list and the ^NSEI benchmark from
// Yahoo into the SQLite store, or updates them nightly. It runs as its own
// process; the server can keep reading the database meanwhile (WAL).
//
//	ingest -db kanso.db -universe NIFTY500 -constituents ind_nifty500list.csv [-from 2005-01-01] [-to YYYY-MM-DD] [-update] [-only-if-behind]
//
// -update fetches each symbol from its last stored date minus 10 days and
// refetches the full history after a split or restatement. A full fetch
// (a backfill or a refetch) starts at -from or the first stored bar,
// whichever is earlier, so stored history is never dropped.
// -only-if-behind (with -update, for the morning run) exits 0 without
// fetching unless an instrument is behind ^NSEI or ^NSEI has no bar for
// the previous weekday.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"kanso/internal/data"
	"kanso/internal/ingest"
	"kanso/internal/market"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "ingest:", err)
		os.Exit(1)
	}
}

func run() error {
	dbPath := flag.String("db", "", "SQLite database file (created if missing)")
	universe := flag.String("universe", "", "stock list name, e.g. NIFTY500")
	constituents := flag.String("constituents", "", "NSE constituents CSV (niftyindices.com) with a Symbol column")
	from := flag.String("from", "2005-01-01", "first date of a full history (YYYY-MM-DD)")
	to := flag.String("to", "", "last date to fetch (default: today in IST)")
	update := flag.Bool("update", false, "fetch only recent bars; refetch full history after a split or restatement")
	onlyIfBehind := flag.Bool("only-if-behind", false, "with -update: do nothing unless the data is behind")
	flag.Parse()
	if *dbPath == "" || *universe == "" || *constituents == "" {
		return errors.New("-db, -universe and -constituents are required")
	}
	if *onlyIfBehind && !*update {
		return errors.New("-only-if-behind needs -update")
	}
	today := market.Date(time.Now().In(time.FixedZone("IST", 19800)).Format(market.DateLayout))
	if *to == "" {
		*to = string(today)
	}
	if !market.Date(*from).Valid() || !market.Date(*to).Valid() {
		return fmt.Errorf("bad -from %q or -to %q", *from, *to)
	}

	f, err := os.Open(*constituents)
	if err != nil {
		return err
	}
	symbols, err := data.ReadNSEConstituents(f)
	_ = f.Close()
	if err != nil {
		return fmt.Errorf("%s: %w", *constituents, err)
	}

	st, err := data.Open(*dbPath, false)
	if err != nil {
		return err
	}
	defer st.Close()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))

	if *onlyIfBehind {
		need, reason, err := ingest.NeedsUpdate(ctx, st, *universe, today)
		if err != nil {
			return err
		}
		if !need {
			log.Info("data is current; skipping update")
			return nil
		}
		log.Info("data is behind; updating", "reason", reason)
	}

	start := time.Now()
	opts := ingest.Options{Universe: *universe, Symbols: symbols, From: market.Date(*from), To: market.Date(*to), Log: log}
	runIngest, mode := ingest.Backfill, "backfill"
	if *update {
		runIngest, mode = ingest.Update, "update"
	}
	sum, err := runIngest(ctx, st, &data.Yahoo{}, opts)
	if err != nil {
		return err
	}
	log.Info(mode+" done", "universe", *universe, "instruments", sum.Instruments, "bars", sum.Bars,
		"failed", len(sum.Failed), "failed_symbols", sum.Failed, "elapsed", time.Since(start).Round(time.Second).String())
	fmt.Println(sum.Freshness())
	if sum.TooManyFailures() {
		return fmt.Errorf("%d of %d symbols failed (more than 5%%)", len(sum.Failed), sum.Instruments)
	}
	return nil
}

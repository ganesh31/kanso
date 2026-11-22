// Command export-fixture writes stocks' daily bars from the SQLite store to
// a Parquet file: the frozen fixture the vectorbt benchmark and its Go
// comparison test both read (spec §8). The store is opened read-only.
//
//	export-fixture -db kanso.db -symbols TCS,INFY,^NSEI -from 2005-01-01 -to 2026-11-20 -out testdata/benchmark/bars.parquet
//
// With -import it runs in reverse: it builds a SQLite store from a Parquet
// fixture and seeds a universe (NIFTY50 by default) with the fixture's
// stocks. The end-to-end tests use this to run the server without the
// network.
//
//	export-fixture -import testdata/benchmark/bars.parquet -db /tmp/e2e.db
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"kanso/internal/data"
	"kanso/internal/market"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "export-fixture:", err)
		os.Exit(1)
	}
}

func run() error {
	dbPath := flag.String("db", "", "SQLite database file")
	symbols := flag.String("symbols", "", "comma-separated symbols, e.g. TCS,INFY,^NSEI")
	from := flag.String("from", "2005-01-01", "first date (YYYY-MM-DD)")
	to := flag.String("to", "", "last date (YYYY-MM-DD); fixed, so the fixture is reproducible")
	out := flag.String("out", "", "Parquet file to write")
	in := flag.String("import", "", "Parquet fixture to load into -db instead of exporting")
	universe := flag.String("universe", "NIFTY50", "with -import: universe to seed with the fixture's stocks")
	flag.Parse()
	if *in != "" {
		if *dbPath == "" {
			return errors.New("-import needs -db")
		}
		return importFixture(*in, *dbPath, *universe)
	}
	if *dbPath == "" || *symbols == "" || *to == "" || *out == "" {
		return errors.New("-db, -symbols, -to and -out are required")
	}
	if !market.Date(*from).Valid() || !market.Date(*to).Valid() {
		return fmt.Errorf("bad -from %q or -to %q", *from, *to)
	}
	var syms []string
	for s := range strings.SplitSeq(*symbols, ",") {
		if s = strings.TrimSpace(s); s != "" {
			syms = append(syms, s)
		}
	}

	st, err := data.Open(*dbPath, true)
	if err != nil {
		return err
	}
	defer st.Close()
	f, err := os.Create(*out)
	if err != nil {
		return err
	}
	if err := st.ExportParquet(context.Background(), syms, market.Date(*from), market.Date(*to), f); err != nil {
		_ = f.Close()
		_ = os.Remove(*out)
		return err
	}
	return f.Close()
}

func importFixture(in, dbPath, universe string) error {
	f, err := os.Open(in)
	if err != nil {
		return err
	}
	defer f.Close()
	st, err := data.Open(dbPath, false)
	if err != nil {
		return err
	}
	if err := st.ImportParquet(context.Background(), f, universe); err != nil {
		_ = st.Close()
		return err
	}
	return st.Close()
}

package market

import (
	"context"
	"errors"
)

// Benchmark is the symbol of the benchmark index (Nifty 50). Its trading
// dates are the calendar data gaps are measured against.
const Benchmark = "^NSEI"

// ErrNotFound is returned (wrapped) when a symbol, instrument or universe
// does not exist.
var ErrNotFound = errors.New("not found")

// Provider loads instruments, bars and universes. SQLite implements it in
// production; MemProvider implements it in tests and for CSV files.
type Provider interface {
	// Instrument looks up a symbol. Unknown symbols return ErrNotFound.
	Instrument(ctx context.Context, symbol string) (Instrument, error)
	// Daily returns the instrument's daily bars with from <= date <= to,
	// sorted ascending.
	Daily(ctx context.Context, id int64, from, to Date) ([]Bar, error)
	// Universe returns a stock list's members, and whether the list has
	// historical membership data (false means survivorship bias).
	Universe(ctx context.Context, name string) (members []Instrument, hasHistory bool, err error)
	// Universes returns every stock list name, sorted.
	Universes(ctx context.Context) ([]string, error)
}

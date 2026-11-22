// Package data is Kanso's storage and data sources: the SQLite store
// (ADR 0002), the Yahoo chart source and the NSE constituent lists.
package data

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"time"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite" // registers the pure-Go "sqlite" driver

	"kanso/internal/data/db"
	"kanso/internal/market"
)

//go:embed migrations/*.sql
var migrations embed.FS

// ErrReadOnly is returned by every write on a store opened read-only.
var ErrReadOnly = errors.New("store is read-only")

// Split is one split or bonus event: Numerator new shares for every
// Denominator old ones, effective on Date.
type Split struct {
	Date        market.Date `json:"date"`
	Numerator   float64     `json:"numerator"`
	Denominator float64     `json:"denominator"`
}

// IngestEntry is one row of the ingest log.
type IngestEntry struct {
	LoggedAt      string      `json:"logged_at"`
	Symbol        string      `json:"symbol"`
	Status        string      `json:"status"`
	Message       string      `json:"message"`
	LatestBarDate market.Date `json:"latest_bar_date"`
}

// Store is the SQLite database in WAL mode. It implements market.Provider
// and is safe for concurrent use.
type Store struct {
	db       *sql.DB
	q        *db.Queries
	readOnly bool
}

var _ market.Provider = (*Store)(nil)

// Open opens the database at path. A read-write open creates the file if
// needed and runs the embedded migrations; a read-only open needs an
// existing, migrated file and rejects writes.
func Open(path string, readOnly bool) (*Store, error) {
	q := url.Values{}
	q.Add("_pragma", "busy_timeout(5000)")
	q.Add("_pragma", "foreign_keys(on)")
	if readOnly {
		if _, err := os.Stat(path); err != nil {
			return nil, fmt.Errorf("open %s: %w", path, err)
		}
		q.Set("mode", "ro")
	} else {
		q.Add("_pragma", "journal_mode(WAL)")
		q.Add("_pragma", "synchronous(NORMAL)")
		// Writers take the write lock when the transaction starts, so two
		// writers wait on busy_timeout instead of deadlocking on upgrade.
		q.Set("_txlock", "immediate")
	}
	conn, err := sql.Open("sqlite", "file:"+path+"?"+q.Encode())
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	if err := conn.Ping(); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	if !readOnly {
		if err := migrate(conn); err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("migrate %s: %w", path, err)
		}
	}
	return &Store{db: conn, q: db.New(conn), readOnly: readOnly}, nil
}

func migrate(conn *sql.DB) error {
	dir, err := fs.Sub(migrations, "migrations")
	if err != nil {
		return err
	}
	p, err := goose.NewProvider(goose.DialectSQLite3, conn, dir)
	if err != nil {
		return err
	}
	_, err = p.Up(context.Background())
	return err
}

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }

func (s *Store) writable() error {
	if s.readOnly {
		return ErrReadOnly
	}
	return nil
}

// inTx runs fn in one write transaction, rolled back if fn fails.
func (s *Store) inTx(ctx context.Context, fn func(q *db.Queries) error) error {
	if err := s.writable(); err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(s.q.WithTx(tx)); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// UpsertInstrument inserts or updates an instrument keyed by (exchange,
// symbol) and returns its id.
func (s *Store) UpsertInstrument(ctx context.Context, in market.Instrument) (int64, error) {
	if err := s.writable(); err != nil {
		return 0, err
	}
	lot := in.LotSize
	if lot == 0 {
		lot = 1
	}
	id, err := s.q.UpsertInstrument(ctx, db.UpsertInstrumentParams{
		Exchange: in.Exchange, Symbol: in.Symbol, Type: string(in.Type), LotSize: int64(lot),
	})
	if err != nil {
		return 0, fmt.Errorf("upsert instrument %s: %w", in.Symbol, err)
	}
	return id, nil
}

// UpsertBars writes bars in one transaction, replacing any stored bar with
// the same (instrument, date).
func (s *Store) UpsertBars(ctx context.Context, id int64, bars []market.Bar) error {
	if err := s.inTx(ctx, func(q *db.Queries) error { return upsertBars(ctx, q, id, bars) }); err != nil {
		return fmt.Errorf("upsert bars of %d: %w", id, err)
	}
	return nil
}

// UpsertDaily writes a fetch's bars and split events in one transaction,
// so a failure leaves neither behind.
func (s *Store) UpsertDaily(ctx context.Context, id int64, bars []market.Bar, splits []Split) error {
	err := s.inTx(ctx, func(q *db.Queries) error {
		if err := upsertBars(ctx, q, id, bars); err != nil {
			return err
		}
		return upsertSplits(ctx, q, id, splits)
	})
	if err != nil {
		return fmt.Errorf("upsert daily of %d: %w", id, err)
	}
	return nil
}

func upsertBars(ctx context.Context, q *db.Queries, id int64, bars []market.Bar) error {
	for _, b := range bars {
		if err := q.UpsertBar(ctx, db.UpsertBarParams(barParams(id, b))); err != nil {
			return fmt.Errorf("bar %s: %w", b.Date, err)
		}
	}
	return nil
}

// ReplaceBars replaces all of an instrument's bars in one transaction: on
// any error the old bars are left untouched.
func (s *Store) ReplaceBars(ctx context.Context, id int64, bars []market.Bar) error {
	err := s.inTx(ctx, func(q *db.Queries) error {
		if err := q.DeleteBars(ctx, id); err != nil {
			return err
		}
		for _, b := range bars {
			if err := q.InsertBar(ctx, barParams(id, b)); err != nil {
				return fmt.Errorf("bar %s: %w", b.Date, err)
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("replace bars of %d: %w", id, err)
	}
	return nil
}

func barParams(id int64, b market.Bar) db.InsertBarParams {
	return db.InsertBarParams{
		InstrumentID: id, Date: string(b.Date),
		Open: b.Open, High: b.High, Low: b.Low, Close: b.Close, Volume: b.Volume,
	}
}

// SaveSplits stores split events, replacing any on the same date.
func (s *Store) SaveSplits(ctx context.Context, id int64, splits []Split) error {
	return s.inTx(ctx, func(q *db.Queries) error { return upsertSplits(ctx, q, id, splits) })
}

func upsertSplits(ctx context.Context, q *db.Queries, id int64, splits []Split) error {
	for _, sp := range splits {
		err := q.UpsertSplit(ctx, db.UpsertSplitParams{
			InstrumentID: id, Date: string(sp.Date), Numerator: sp.Numerator, Denominator: sp.Denominator,
		})
		if err != nil {
			return fmt.Errorf("split %s: %w", sp.Date, err)
		}
	}
	return nil
}

// Splits returns an instrument's stored split events, oldest first.
func (s *Store) Splits(ctx context.Context, id int64) ([]Split, error) {
	rows, err := s.q.Splits(ctx, id)
	if err != nil {
		return nil, err
	}
	out := make([]Split, len(rows))
	for i, r := range rows {
		out[i] = Split{Date: market.Date(r.Date), Numerator: r.Numerator, Denominator: r.Denominator}
	}
	return out, nil
}

// LatestBarDate returns the date of the instrument's last stored bar, or
// "" when it has none.
func (s *Store) LatestBarDate(ctx context.Context, id int64) (market.Date, error) {
	d, err := s.q.LatestBarDate(ctx, id)
	return market.Date(d), err
}

// ReplaceUniverse makes ids the members of the named universe (creating
// it if needed). Members are written with from_date NULL: their history
// is unknown.
func (s *Store) ReplaceUniverse(ctx context.Context, name string, ids []int64) error {
	return s.inTx(ctx, func(q *db.Queries) error {
		uid, err := q.UpsertUniverse(ctx, name)
		if err != nil {
			return err
		}
		if err := q.DeleteUniverseMembers(ctx, uid); err != nil {
			return err
		}
		for _, id := range ids {
			if err := q.InsertUniverseMember(ctx, db.InsertUniverseMemberParams{UniverseID: uid, InstrumentID: id}); err != nil {
				return fmt.Errorf("universe %s member %d: %w", name, id, err)
			}
		}
		return nil
	})
}

// LogIngest records one symbol's ingest outcome with its latest stored bar
// date.
func (s *Store) LogIngest(ctx context.Context, id int64, status, msg string) error {
	if err := s.writable(); err != nil {
		return err
	}
	latest, err := s.LatestBarDate(ctx, id)
	if err != nil {
		return err
	}
	return s.q.LogIngest(ctx, db.LogIngestParams{
		InstrumentID:  id,
		LoggedAt:      time.Now().UTC().Format(time.RFC3339),
		Status:        status,
		Message:       msg,
		LatestBarDate: sql.NullString{String: string(latest), Valid: latest != ""},
	})
}

// IngestLog returns every ingest log entry, oldest first.
func (s *Store) IngestLog(ctx context.Context) ([]IngestEntry, error) {
	rows, err := s.q.IngestLog(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]IngestEntry, len(rows))
	for i, r := range rows {
		out[i] = IngestEntry{
			LoggedAt: r.LoggedAt, Symbol: r.Symbol, Status: r.Status, Message: r.Message,
			LatestBarDate: market.Date(r.LatestBarDate.String),
		}
	}
	return out, nil
}

// Instrument implements market.Provider.
func (s *Store) Instrument(ctx context.Context, symbol string) (market.Instrument, error) {
	r, err := s.q.InstrumentBySymbol(ctx, symbol)
	if errors.Is(err, sql.ErrNoRows) {
		return market.Instrument{}, fmt.Errorf("instrument %q: %w", symbol, market.ErrNotFound)
	}
	if err != nil {
		return market.Instrument{}, fmt.Errorf("instrument %q: %w", symbol, err)
	}
	return instrument(r.ID, r.Exchange, r.Symbol, r.Type, r.LotSize), nil
}

func instrument(id int64, exchange, symbol, typ string, lot int64) market.Instrument {
	return market.Instrument{ID: id, Exchange: exchange, Symbol: symbol, Type: market.InstrumentType(typ), LotSize: int(lot)}
}

// Daily implements market.Provider.
func (s *Store) Daily(ctx context.Context, id int64, from, to market.Date) ([]market.Bar, error) {
	if _, err := s.q.InstrumentByID(ctx, id); errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("instrument id %d: %w", id, market.ErrNotFound)
	} else if err != nil {
		return nil, err
	}
	rows, err := s.q.DailyBars(ctx, db.DailyBarsParams{InstrumentID: id, FromDate: string(from), ToDate: string(to)})
	if err != nil {
		return nil, fmt.Errorf("daily bars of %d: %w", id, err)
	}
	out := make([]market.Bar, len(rows))
	for i, r := range rows {
		out[i] = market.Bar{Date: market.Date(r.Date), Open: r.Open, High: r.High, Low: r.Low, Close: r.Close, Volume: r.Volume}
	}
	return out, nil
}

// Universe implements market.Provider. Members are sorted by symbol; the
// list has history only if some member has a known from_date.
func (s *Store) Universe(ctx context.Context, name string) ([]market.Instrument, bool, error) {
	uid, err := s.q.UniverseID(ctx, name)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, fmt.Errorf("universe %q: %w", name, market.ErrNotFound)
	}
	if err != nil {
		return nil, false, err
	}
	rows, err := s.q.UniverseMembers(ctx, uid)
	if err != nil {
		return nil, false, err
	}
	out := make([]market.Instrument, len(rows))
	hasHistory := false
	for i, r := range rows {
		out[i] = instrument(r.ID, r.Exchange, r.Symbol, r.Type, r.LotSize)
		hasHistory = hasHistory || r.FromDate.Valid
	}
	return out, hasHistory, nil
}

// Universes implements market.Provider.
func (s *Store) Universes(ctx context.Context) ([]string, error) {
	names, err := s.q.UniverseNames(ctx)
	if err != nil {
		return nil, err
	}
	if names == nil {
		names = []string{}
	}
	return names, nil
}

// ExportParquet writes the named symbols' bars in [from, to] to w as
// Parquet (market.WriteParquet). Unknown symbols are an error.
func (s *Store) ExportParquet(ctx context.Context, symbols []string, from, to market.Date, w io.Writer) error {
	bars := make(map[string][]market.Bar, len(symbols))
	for _, sym := range symbols {
		in, err := s.Instrument(ctx, sym)
		if err != nil {
			return err
		}
		if bars[sym], err = s.Daily(ctx, in.ID, from, to); err != nil {
			return err
		}
	}
	return market.WriteParquet(w, bars)
}

-- +goose Up

-- One tradable or reference symbol (spec §6.2).
CREATE TABLE instruments (
    id       INTEGER PRIMARY KEY,
    exchange TEXT    NOT NULL,
    symbol   TEXT    NOT NULL,
    type     TEXT    NOT NULL CHECK (type IN ('equity', 'index', 'future', 'option')),
    lot_size INTEGER NOT NULL DEFAULT 1,
    UNIQUE (exchange, symbol)
);

-- Split-adjusted daily OHLCV; date is the IST trading date (YYYY-MM-DD).
CREATE TABLE bars_daily (
    instrument_id INTEGER NOT NULL REFERENCES instruments (id) ON DELETE CASCADE,
    date          TEXT    NOT NULL,
    open          REAL    NOT NULL,
    high          REAL    NOT NULL,
    low           REAL    NOT NULL,
    close         REAL    NOT NULL,
    volume        REAL    NOT NULL,
    PRIMARY KEY (instrument_id, date)
) WITHOUT ROWID;

-- Split and bonus events as the source reported them.
CREATE TABLE splits (
    instrument_id INTEGER NOT NULL REFERENCES instruments (id) ON DELETE CASCADE,
    date          TEXT    NOT NULL,
    numerator     REAL    NOT NULL,
    denominator   REAL    NOT NULL,
    PRIMARY KEY (instrument_id, date)
) WITHOUT ROWID;

-- Stock lists. from_date NULL means the membership start is unknown
-- (survivorship bias, spec §6.3).
CREATE TABLE universes (
    id   INTEGER PRIMARY KEY,
    name TEXT    NOT NULL UNIQUE
);

CREATE TABLE universe_members (
    universe_id   INTEGER NOT NULL REFERENCES universes (id) ON DELETE CASCADE,
    instrument_id INTEGER NOT NULL REFERENCES instruments (id) ON DELETE CASCADE,
    from_date     TEXT,
    to_date       TEXT
);

CREATE INDEX universe_members_universe ON universe_members (universe_id);

-- Saved systems: the rule tree as JSON (spec §7.3).
CREATE TABLE systems (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    definition TEXT NOT NULL,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

-- Created for later use; V1 does not write to it.
CREATE TABLE backtest_runs (
    id         INTEGER PRIMARY KEY,
    system_id  TEXT REFERENCES systems (id) ON DELETE SET NULL,
    created_at TEXT NOT NULL,
    request    TEXT NOT NULL,
    result     TEXT NOT NULL
);

-- Per-symbol ingest outcomes (spec §6.4).
CREATE TABLE ingest_log (
    id              INTEGER PRIMARY KEY,
    instrument_id   INTEGER NOT NULL REFERENCES instruments (id) ON DELETE CASCADE,
    logged_at       TEXT    NOT NULL,
    status          TEXT    NOT NULL,
    message         TEXT    NOT NULL,
    latest_bar_date TEXT
);

CREATE INDEX ingest_log_instrument ON ingest_log (instrument_id, logged_at);

-- +goose Down
DROP TABLE ingest_log;
DROP TABLE backtest_runs;
DROP TABLE systems;
DROP TABLE universe_members;
DROP TABLE universes;
DROP TABLE splits;
DROP TABLE bars_daily;
DROP TABLE instruments;

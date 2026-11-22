-- name: UpsertBar :exec
INSERT INTO bars_daily (instrument_id, date, open, high, low, close, volume)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (instrument_id, date) DO UPDATE
SET open = excluded.open, high = excluded.high, low = excluded.low,
    close = excluded.close, volume = excluded.volume;

-- name: InsertBar :exec
INSERT INTO bars_daily (instrument_id, date, open, high, low, close, volume)
VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: DeleteBars :exec
DELETE FROM bars_daily WHERE instrument_id = ?;

-- name: DailyBars :many
SELECT date, open, high, low, close, volume
FROM bars_daily
WHERE instrument_id = sqlc.arg(instrument_id)
  AND date >= sqlc.arg(from_date) AND date <= sqlc.arg(to_date)
ORDER BY date;

-- name: LatestBarDate :one
SELECT CAST(COALESCE(MAX(date), '') AS TEXT) AS latest
FROM bars_daily
WHERE instrument_id = ?;

-- name: EarliestBarDate :one
SELECT CAST(COALESCE(MIN(date), '') AS TEXT) AS earliest
FROM bars_daily
WHERE instrument_id = ?;

-- name: UpsertSplit :exec
INSERT INTO splits (instrument_id, date, numerator, denominator)
VALUES (?, ?, ?, ?)
ON CONFLICT (instrument_id, date) DO UPDATE
SET numerator = excluded.numerator, denominator = excluded.denominator;

-- name: Splits :many
SELECT date, numerator, denominator
FROM splits
WHERE instrument_id = ?
ORDER BY date;

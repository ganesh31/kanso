-- name: UpsertInstrument :one
INSERT INTO instruments (exchange, symbol, type, lot_size)
VALUES (?, ?, ?, ?)
ON CONFLICT (exchange, symbol) DO UPDATE
SET type = excluded.type, lot_size = excluded.lot_size
RETURNING id;

-- name: InstrumentBySymbol :one
SELECT id, exchange, symbol, type, lot_size
FROM instruments
WHERE symbol = ?
ORDER BY id
LIMIT 1;

-- name: InstrumentByID :one
SELECT id, exchange, symbol, type, lot_size
FROM instruments
WHERE id = ?;

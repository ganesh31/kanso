-- name: UpsertUniverse :one
INSERT INTO universes (name) VALUES (?)
ON CONFLICT (name) DO UPDATE SET name = excluded.name
RETURNING id;

-- name: UniverseID :one
SELECT id FROM universes WHERE name = ?;

-- name: DeleteUniverseMembers :exec
DELETE FROM universe_members WHERE universe_id = ?;

-- name: InsertUniverseMember :exec
INSERT INTO universe_members (universe_id, instrument_id, from_date, to_date)
VALUES (?, ?, NULL, NULL);

-- name: UniverseMembers :many
SELECT i.id, i.exchange, i.symbol, i.type, i.lot_size, m.from_date
FROM universe_members m
JOIN instruments i ON i.id = m.instrument_id
WHERE m.universe_id = ? AND m.to_date IS NULL
ORDER BY i.symbol;

-- name: UniverseNames :many
SELECT name FROM universes ORDER BY name;

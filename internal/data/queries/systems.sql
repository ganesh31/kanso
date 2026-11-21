-- name: UpsertSystem :exec
INSERT INTO systems (id, name, definition, created_at, updated_at)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT (id) DO UPDATE SET
    name = excluded.name,
    definition = excluded.definition,
    updated_at = excluded.updated_at;

-- name: GetSystem :one
SELECT definition FROM systems WHERE id = ?;

-- name: ListSystems :many
SELECT definition FROM systems ORDER BY name, id;

-- name: DeleteSystem :execrows
DELETE FROM systems WHERE id = ?;

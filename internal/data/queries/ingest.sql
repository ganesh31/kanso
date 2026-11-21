-- name: LogIngest :exec
INSERT INTO ingest_log (instrument_id, logged_at, status, message, latest_bar_date)
VALUES (?, ?, ?, ?, ?);

-- name: IngestLog :many
SELECT l.logged_at, i.symbol, l.status, l.message, l.latest_bar_date
FROM ingest_log l
JOIN instruments i ON i.id = l.instrument_id
ORDER BY l.id;

package data

import (
	"database/sql"
	"fmt"
	"os"
)

// Compact turns the database at path into one self-contained file for the
// demo image: it checkpoints the WAL, switches to a rollback journal and
// vacuums. Nothing else may have the file open. A rollback-journal file
// opens read-only without the -shm file WAL needs, so it works on a
// read-only filesystem.
func Compact(path string) error {
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("compact %s: %w", path, err)
	}
	conn, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		return fmt.Errorf("compact %s: %w", path, err)
	}
	// One connection: journal_mode applies to the connection that sets it.
	conn.SetMaxOpenConns(1)
	for _, stmt := range []string{
		"PRAGMA wal_checkpoint(TRUNCATE)",
		"PRAGMA journal_mode=DELETE",
		"VACUUM",
	} {
		if _, err := conn.Exec(stmt); err != nil {
			_ = conn.Close()
			return fmt.Errorf("compact %s: %s: %w", path, stmt, err)
		}
	}
	if err := conn.Close(); err != nil {
		return fmt.Errorf("compact %s: %w", path, err)
	}
	return nil
}

package data

import (
	"context"
	"os"
	"testing"
)

func TestCompactLeavesOneRollbackJournalFile(t *testing.T) {
	ctx := context.Background()
	s, path := openTemp(t)
	id := addInstrument(t, s, "TCS")
	if err := s.UpsertBars(ctx, id, bars(50, 10)); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	if err := Compact(path); err != nil {
		t.Fatal(err)
	}
	for _, side := range []string{"-wal", "-shm", "-journal"} {
		if _, err := os.Stat(path + side); !os.IsNotExist(err) {
			t.Errorf("%s left behind (err %v)", side, err)
		}
	}
	// SQLite file header bytes 18 and 19 are the read and write format
	// versions: 1 for a rollback journal, 2 for WAL.
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if b[18] != 1 || b[19] != 1 {
		t.Fatalf("format versions %d/%d, want 1/1 (rollback journal)", b[18], b[19])
	}

	// The snapshot opens read-only with its data.
	ro, err := Open(path, true)
	if err != nil {
		t.Fatal(err)
	}
	defer ro.Close()
	inst, err := ro.Instrument(ctx, "TCS")
	if err != nil {
		t.Fatal(err)
	}
	got, err := ro.Daily(ctx, inst.ID, "", "9999-12-31")
	if err != nil || len(got) != 50 {
		t.Fatalf("daily: %d bars, %v", len(got), err)
	}
}

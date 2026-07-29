package crawlstate

import (
	"path/filepath"
	"testing"
)

func TestDeletionLog_DeleteThenIsDeleted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deletions.jsonl")

	d, err := OpenDeletionLog(path)
	if err != nil {
		t.Fatalf("OpenDeletionLog returned error: %v", err)
	}
	defer d.Close()

	if d.IsDeleted("abc123") {
		t.Fatal("IsDeleted was true before any deletion was recorded")
	}
	if err := d.Delete("abc123", "user requested removal"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if !d.IsDeleted("abc123") {
		t.Error("IsDeleted was false immediately after Delete")
	}
}

func TestDeletionLog_UnrelatedIDIsNotDeleted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deletions.jsonl")

	d, _ := OpenDeletionLog(path)
	defer d.Close()

	d.Delete("abc123", "reason")
	if d.IsDeleted("xyz789") {
		t.Error("an unrelated passage ID was reported as deleted")
	}
}

func TestDeletionLog_SurvivesReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deletions.jsonl")

	d1, err := OpenDeletionLog(path)
	if err != nil {
		t.Fatalf("OpenDeletionLog returned error: %v", err)
	}
	d1.Delete("abc123", "user requested removal")
	if err := d1.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	// Simulate a restart: fresh in-memory state, same log file on disk.
	d2, err := OpenDeletionLog(path)
	if err != nil {
		t.Fatalf("reopening returned error: %v", err)
	}
	defer d2.Close()

	if !d2.IsDeleted("abc123") {
		t.Error("a deletion recorded before restart was not recognized after reopen")
	}
	if d2.IsDeleted("never-deleted") {
		t.Error("an unrelated ID was reported as deleted after reopen")
	}
}

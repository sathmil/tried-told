package wal

import (
	"os"
	"path/filepath"
	"testing"
)

type entry struct {
	Value string `json:"value"`
}

func TestLog_AppendThenReplayRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.jsonl")

	l, err := Open[entry](path)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	want := []entry{{Value: "a"}, {Value: "b"}, {Value: "c"}}
	for _, e := range want {
		if err := l.Append(e); err != nil {
			t.Fatalf("Append(%v) returned error: %v", e, err)
		}
	}
	if err := l.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	var got []entry
	if err := Replay[entry](path, func(e entry) { got = append(got, e) }); err != nil {
		t.Fatalf("Replay returned error: %v", err)
	}

	if len(got) != len(want) {
		t.Fatalf("replayed %d entries, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry %d = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestReplay_MissingFileIsNotAnError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.jsonl")

	called := false
	if err := Replay[entry](path, func(entry) { called = true }); err != nil {
		t.Fatalf("Replay on a missing file returned error: %v", err)
	}
	if called {
		t.Error("Replay called fn even though the log doesn't exist")
	}
}

func TestReplay_TornTrailingLineIsRecoveredFrom(t *testing.T) {
	path := filepath.Join(t.TempDir(), "torn.jsonl")

	// Simulate a crash mid-write: two complete, valid lines, then a
	// truncated/malformed final line (as if the process died mid-Write).
	content := `{"value":"a"}
{"value":"b"}
{"value":"c"` // note: no closing brace/quote - deliberately malformed
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test fixture: %v", err)
	}

	var got []entry
	if err := Replay[entry](path, func(e entry) { got = append(got, e) }); err != nil {
		t.Fatalf("Replay returned error on a torn trailing line: %v", err)
	}

	want := []entry{{Value: "a"}, {Value: "b"}}
	if len(got) != len(want) {
		t.Fatalf("replayed %d entries, want %d (everything before the torn line)", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry %d = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestLog_ReopenAfterCloseAppendsRatherThanOverwrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "resume.jsonl")

	l1, _ := Open[entry](path)
	l1.Append(entry{Value: "first"})
	l1.Close()

	l2, err := Open[entry](path)
	if err != nil {
		t.Fatalf("reopening returned error: %v", err)
	}
	l2.Append(entry{Value: "second"})
	l2.Close()

	var got []entry
	Replay[entry](path, func(e entry) { got = append(got, e) })

	want := []entry{{Value: "first"}, {Value: "second"}}
	if len(got) != len(want) {
		t.Fatalf("replayed %d entries, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry %d = %v, want %v", i, got[i], want[i])
		}
	}
}

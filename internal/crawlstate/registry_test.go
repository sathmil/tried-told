package crawlstate

import (
	"path/filepath"
	"testing"
)

func TestPersistentRegistry_SeenStateSurvivesReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "seen.jsonl")

	pr1, err := OpenRegistry(path, 100, 0.01)
	if err != nil {
		t.Fatalf("OpenRegistry returned error: %v", err)
	}
	if pr1.SeenOrAdd("https://a.com/1") {
		t.Fatal("first SeenOrAdd for a new URL returned true")
	}
	if err := pr1.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	// Simulate a restart: fresh PersistentRegistry, same log, no carried
	// over in-memory state.
	pr2, err := OpenRegistry(path, 100, 0.01)
	if err != nil {
		t.Fatalf("reopening returned error: %v", err)
	}
	defer pr2.Close()

	if !pr2.SeenOrAdd("https://a.com/1") {
		t.Error("previously-seen URL was not recognized as seen after reopen")
	}
	if pr2.SeenOrAdd("https://a.com/2") {
		t.Error("a genuinely new URL was reported as already seen after reopen")
	}
}

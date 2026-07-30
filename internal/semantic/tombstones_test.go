package semantic

import (
	"path/filepath"
	"testing"

	"triedandtold/internal/crawlstate"
	"triedandtold/internal/embeddings"
)

func openDeletionLog(t *testing.T) *crawlstate.DeletionLog {
	t.Helper()
	log, err := crawlstate.OpenDeletionLog(filepath.Join(t.TempDir(), "deletions.jsonl"))
	if err != nil {
		t.Fatalf("OpenDeletionLog returned error: %v", err)
	}
	t.Cleanup(func() { log.Close() })
	return log
}

func TestFilterDeleted_ExcludesDeletedID(t *testing.T) {
	log := openDeletionLog(t)
	if err := log.Delete("b", "test deletion"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}

	got := FilterDeleted([]string{"a", "b", "c"}, log)
	want := []string{"a", "c"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("FilterDeleted(...) = %v, want %v", got, want)
	}
}

func TestFilterDeleted_NoDeletionsPreservesOrder(t *testing.T) {
	log := openDeletionLog(t)
	in := []string{"a", "b", "c"}
	got := FilterDeleted(in, log)
	if len(got) != 3 || got[0] != "a" || got[1] != "b" || got[2] != "c" {
		t.Errorf("FilterDeleted(...) = %v, want %v unchanged", got, in)
	}
}

// TestSearchLive_OverFetchCompensatesForDeletedTopResults is the real
// proof: the two nearest neighbors to the query are both tombstoned, but
// SearchLive still returns k=2 live results, by reaching past them into
// the over-fetched candidates - which a plain Search(query, k) +
// FilterDeleted would not do, since Search only ever returns k
// candidates to begin with.
func TestSearchLive_OverFetchCompensatesForDeletedTopResults(t *testing.T) {
	idx, err := Open(filepath.Join(t.TempDir(), "index.graph"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}

	query := []float32{1, 0, 0}
	if err := idx.Add(&embeddings.File{Embeddings: []embeddings.Embedding{
		{PassageID: "a", Vector: []float32{1, 0, 0}},       // nearest
		{PassageID: "b", Vector: []float32{0.99, 0.01, 0}}, // 2nd nearest
		{PassageID: "c", Vector: []float32{0.9, 0.1, 0}},   // 3rd nearest
		{PassageID: "d", Vector: []float32{0.5, 0.5, 0}},   // 4th nearest
		{PassageID: "e", Vector: []float32{0, 1, 0}},       // farthest
	}}); err != nil {
		t.Fatalf("Add returned error: %v", err)
	}

	log := openDeletionLog(t)
	log.Delete("a", "test deletion")
	log.Delete("b", "test deletion")

	// A naive Search(query, 2) would only ever see "a" and "b" - both
	// deleted - and filtering would leave nothing. SearchLive must
	// reach further into the graph instead.
	got := idx.SearchLive(query, 2, log)
	if len(got) != 2 {
		t.Fatalf("SearchLive(...) = %v, want 2 live results (c and d)", got)
	}
	if got[0] != "c" || got[1] != "d" {
		t.Errorf("SearchLive(...) = %v, want [c d]", got)
	}
}

// TestSearchLive_ReturnsFewerThanKWhenNotEnoughLiveResultsExist confirms
// the accepted, honest limitation: over-fetching narrows the gap between
// requested and returned results, it doesn't guarantee closing it. No
// padding or fabricating results to hit k.
func TestSearchLive_ReturnsFewerThanKWhenNotEnoughLiveResultsExist(t *testing.T) {
	idx, err := Open(filepath.Join(t.TempDir(), "index.graph"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if err := idx.Add(&embeddings.File{Embeddings: []embeddings.Embedding{
		{PassageID: "a", Vector: []float32{1, 0, 0}},
		{PassageID: "b", Vector: []float32{0.9, 0.1, 0}},
		{PassageID: "c", Vector: []float32{0, 1, 0}},
	}}); err != nil {
		t.Fatalf("Add returned error: %v", err)
	}

	log := openDeletionLog(t)
	log.Delete("a", "test deletion")
	log.Delete("b", "test deletion")

	got := idx.SearchLive([]float32{1, 0, 0}, 2, log)
	if len(got) != 1 || got[0] != "c" {
		t.Errorf("SearchLive(...) = %v, want exactly [c] (only one live passage exists)", got)
	}
}

package semantic

import (
	"path/filepath"
	"testing"

	"triedandtold/internal/embeddings"
)

func TestIndex_FindsExactMatchWithSyntheticVectors(t *testing.T) {
	idx, err := Open(filepath.Join(t.TempDir(), "index.graph"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}

	f := &embeddings.File{Embeddings: []embeddings.Embedding{
		{PassageID: "a", Vector: []float32{1, 0, 0}},
		{PassageID: "b", Vector: []float32{0, 1, 0}},
		{PassageID: "c", Vector: []float32{0, 0, 1}},
	}}
	if err := idx.Add(f); err != nil {
		t.Fatalf("Add returned error: %v", err)
	}

	if idx.Len() != 3 {
		t.Errorf("Len() = %d, want 3", idx.Len())
	}

	results := idx.Search([]float32{1, 0, 0}, 1)
	if len(results) != 1 || results[0] != "a" {
		t.Errorf("Search([1,0,0], 1) = %v, want [a]", results)
	}
}

func TestIndex_PersistsAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "index.graph")

	idx1, err := Open(path)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if err := idx1.Add(&embeddings.File{Embeddings: []embeddings.Embedding{
		{PassageID: "a", Vector: []float32{1, 0, 0}},
	}}); err != nil {
		t.Fatalf("Add returned error: %v", err)
	}

	idx2, err := Open(path)
	if err != nil {
		t.Fatalf("reopening returned error: %v", err)
	}
	if idx2.Len() != 1 {
		t.Errorf("Len() after reopen = %d, want 1", idx2.Len())
	}
	results := idx2.Search([]float32{1, 0, 0}, 1)
	if len(results) != 1 || results[0] != "a" {
		t.Errorf("Search after reopen = %v, want [a]", results)
	}
}

func TestIndex_IncrementalAddFindsNewlyAddedVectors(t *testing.T) {
	idx, err := Open(filepath.Join(t.TempDir(), "index.graph"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}

	idx.Add(&embeddings.File{Embeddings: []embeddings.Embedding{
		{PassageID: "a", Vector: []float32{1, 0, 0}},
	}})
	if idx.Len() != 1 {
		t.Fatalf("Len() after first Add = %d, want 1", idx.Len())
	}

	// A second, independent Add - simulating a later incremental batch,
	// not a rebuild.
	idx.Add(&embeddings.File{Embeddings: []embeddings.Embedding{
		{PassageID: "b", Vector: []float32{0, 1, 0}},
	}})
	if idx.Len() != 2 {
		t.Fatalf("Len() after second Add = %d, want 2", idx.Len())
	}

	results := idx.Search([]float32{0, 1, 0}, 1)
	if len(results) != 1 || results[0] != "b" {
		t.Errorf("Search([0,1,0], 1) = %v, want [b] (the incrementally-added vector)", results)
	}
}

func TestIndex_ReAddingSameIDReplacesRatherThanDuplicates(t *testing.T) {
	idx, err := Open(filepath.Join(t.TempDir(), "index.graph"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}

	idx.Add(&embeddings.File{Embeddings: []embeddings.Embedding{
		{PassageID: "a", Vector: []float32{1, 0, 0}},
	}})
	idx.Add(&embeddings.File{Embeddings: []embeddings.Embedding{
		{PassageID: "a", Vector: []float32{0, 1, 0}}, // same ID, new vector
	}})

	if idx.Len() != 1 {
		t.Errorf("Len() = %d, want 1 (re-adding the same ID must replace, not duplicate)", idx.Len())
	}
	results := idx.Search([]float32{0, 1, 0}, 1)
	if len(results) != 1 || results[0] != "a" {
		t.Errorf("Search([0,1,0], 1) = %v, want [a] (should find the updated vector)", results)
	}
}

// TestIndex_DuplicateIDWithinOneBatchDoesNotPanic covers a case the
// re-add path doesn't: two embeddings sharing the same PassageID
// arriving in the *same* Add call (a real crawl produced this - a
// decorative element repeated verbatim on one page hashes identically).
// Neither exists in the graph yet, so naively queuing both into one
// underlying Add call hits coder/hnsw's duplicate-key panic just as
// surely as re-adding an existing key does.
func TestIndex_DuplicateIDWithinOneBatchDoesNotPanic(t *testing.T) {
	idx, err := Open(filepath.Join(t.TempDir(), "index.graph"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}

	err = idx.Add(&embeddings.File{Embeddings: []embeddings.Embedding{
		{PassageID: "a", Vector: []float32{1, 0, 0}},
		{PassageID: "dup", Vector: []float32{0, 1, 0}},
		{PassageID: "dup", Vector: []float32{0, 0, 1}}, // same ID, later vector wins
	}})
	if err != nil {
		t.Fatalf("Add returned error: %v", err)
	}

	if idx.Len() != 2 {
		t.Fatalf("Len() = %d, want 2 (a duplicate ID within one batch must collapse, not error)", idx.Len())
	}
	results := idx.Search([]float32{0, 0, 1}, 1)
	if len(results) != 1 || results[0] != "dup" {
		t.Errorf("Search([0,0,1], 1) = %v, want [dup] (the later of the two same-ID vectors)", results)
	}
}

// TestIndex_FindsSemanticallyRelevantPassageWithRealEmbeddings is the
// proof that actually matters: real BGE embeddings for three topically
// distinct passages (sunscreen, pizza, car maintenance), searched with a
// real query embedding (with the query instruction prefix applied, unlike
// passage embeddings) for a sunscreen-related question - the sunscreen
// passage must rank first, not just "some result."
func TestIndex_FindsSemanticallyRelevantPassageWithRealEmbeddings(t *testing.T) {
	passages, err := embeddings.Open("testdata/passages.bin")
	if err != nil {
		t.Fatalf("failed to open passages fixture: %v", err)
	}
	query, err := embeddings.Open("testdata/query.bin")
	if err != nil {
		t.Fatalf("failed to open query fixture: %v", err)
	}
	if len(query.Embeddings) != 1 {
		t.Fatalf("query fixture has %d embeddings, want exactly 1", len(query.Embeddings))
	}

	idx, err := Open(filepath.Join(t.TempDir(), "index.graph"))
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}
	if err := idx.Add(passages); err != nil {
		t.Fatalf("Add returned error: %v", err)
	}

	// testdata/passages.bin was built (see the fixture generation notes
	// in docs/design/26) from: a sunscreen passage, a pizza-recipe
	// passage, and a car-maintenance passage, in that order.
	sunscreenID := passages.Embeddings[0].PassageID

	results := idx.Search(query.Embeddings[0].Vector, 1)
	if len(results) != 1 || results[0] != sunscreenID {
		t.Errorf("Search for a sunscreen-related query = %v, want the sunscreen passage %q", results, sunscreenID)
	}
}

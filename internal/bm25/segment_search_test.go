package bm25

import (
	"os"
	"path/filepath"
	"testing"

	"triedandtold/internal/diskindex"
	"triedandtold/internal/extract"
	"triedandtold/internal/index"
)

func TestSearch_AgainstDiskSegment(t *testing.T) {
	passages := []extract.Passage{
		{Text: "sunscreen leaves white cast", SourceURL: "https://a.com/1"},
		{Text: "sunscreen does not leave residue", SourceURL: "https://a.com/2"},
		{Text: "moisturizer is great", SourceURL: "https://a.com/3"},
	}

	path := filepath.Join(t.TempDir(), "segment.idx")
	if err := diskindex.BuildSegment(passages, path); err != nil {
		t.Fatalf("BuildSegment returned error: %v", err)
	}
	seg, err := diskindex.OpenSegment(path)
	if err != nil {
		t.Fatalf("OpenSegment returned error: %v", err)
	}

	results := Search(WrapSegment(seg), "sunscreen", DefaultParams)
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
	for _, r := range results {
		if r.DocID != 0 && r.DocID != 1 {
			t.Errorf("unexpected DocID %d in results (moisturizer passage should be excluded)", r.DocID)
		}
	}

	// The whole point of this test: results reference the segment's own
	// local IDs, which resolve back through the segment's own ID-mapping
	// to the real Passage.ID() - the join that makes deletion/attribution
	// possible against search results, not just against extraction output.
	for _, r := range results {
		gotID := seg.PassageID(r.DocID)
		wantID := passages[r.DocID].ID()
		if gotID != wantID {
			t.Errorf("PassageID(%d) = %q, want %q", r.DocID, gotID, wantID)
		}
	}
}

// TestSearch_InMemoryAndSegmentAgreeOnRealExtractedData is the strongest
// form of this proof: the same real passages, indexed both ways (the
// original in-memory index.Index and a real disk segment), must produce
// the same ranked sequence of actual passages - the disk-backed path isn't
// just "returns something", it reproduces the same, already-tested
// scoring behavior as the backend it's replacing.
func TestSearch_InMemoryAndSegmentAgreeOnRealExtractedData(t *testing.T) {
	html, err := os.ReadFile("../extract/testdata/example_site.html")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}
	passages, err := extract.ExampleSiteExtractor{}.Extract(string(html), "https://example-reviews.test/product/1")
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}

	// In-memory backend, built the Milestone 1 way.
	docs := make([]index.IndexDoc, len(passages))
	for i, p := range passages {
		docs[i] = index.IndexDoc{ID: i, Text: p.Text}
	}
	memIdx := index.BuildIndex(docs)

	// Disk-backed backend, built from the same passages in the same order.
	path := filepath.Join(t.TempDir(), "segment.idx")
	if err := diskindex.BuildSegment(passages, path); err != nil {
		t.Fatalf("BuildSegment returned error: %v", err)
	}
	seg, err := diskindex.OpenSegment(path)
	if err != nil {
		t.Fatalf("OpenSegment returned error: %v", err)
	}

	const query = "sunscreen white cast"
	memResults := Search(WrapInMemory(memIdx), query, DefaultParams)
	segResults := Search(WrapSegment(seg), query, DefaultParams)

	if len(memResults) == 0 {
		t.Fatal("expected at least one result from the in-memory search")
	}
	if len(memResults) != len(segResults) {
		t.Fatalf("got %d in-memory results but %d segment results, want equal", len(memResults), len(segResults))
	}

	// Both backends assign local IDs 0..N-1 by iterating the same
	// passages slice in the same order, so DocIDs should literally match
	// position-for-position - and each should resolve (via its own
	// mapping) back to the identical real passage.
	for i := range memResults {
		if memResults[i].DocID != segResults[i].DocID {
			t.Errorf("result %d: in-memory DocID=%d, segment DocID=%d, want equal", i, memResults[i].DocID, segResults[i].DocID)
		}
		if memResults[i].Score != segResults[i].Score {
			t.Errorf("result %d: in-memory Score=%v, segment Score=%v, want equal", i, memResults[i].Score, segResults[i].Score)
		}

		wantPassageID := passages[segResults[i].DocID].ID()
		if got := seg.PassageID(segResults[i].DocID); got != wantPassageID {
			t.Errorf("result %d: segment PassageID = %q, want %q", i, got, wantPassageID)
		}
	}
}

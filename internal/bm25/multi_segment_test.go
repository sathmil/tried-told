package bm25

import (
	"path/filepath"
	"testing"

	"triedandtold/internal/crawlstate"
	"triedandtold/internal/diskindex"
	"triedandtold/internal/extract"
)

// TestSearch_FindsResultsAcrossIncrementallyAddedSegments is the actual
// point of incremental indexing: passages written as two separate
// segments at two separate times (simulating an initial crawl, then a
// later batch) must both be searchable together, seamlessly, through the
// same WrapSegment(*MultiSegment) as a single Search call.
func TestSearch_FindsResultsAcrossIncrementallyAddedSegments(t *testing.T) {
	// "First crawl": one segment.
	seg1Path := filepath.Join(t.TempDir(), "seg1.idx")
	if err := diskindex.BuildSegment([]extract.Passage{
		{Text: "sunscreen leaves white cast", SourceURL: "https://a.com/1"},
	}, seg1Path); err != nil {
		t.Fatalf("BuildSegment(seg1) returned error: %v", err)
	}

	// "Later crawl": a second, independent segment - no rebuild of seg1.
	seg2Path := filepath.Join(t.TempDir(), "seg2.idx")
	if err := diskindex.BuildSegment([]extract.Passage{
		{Text: "sunscreen does not oxidize", SourceURL: "https://a.com/2"},
	}, seg2Path); err != nil {
		t.Fatalf("BuildSegment(seg2) returned error: %v", err)
	}

	m, err := diskindex.OpenMultiSegment([]string{seg1Path, seg2Path})
	if err != nil {
		t.Fatalf("OpenMultiSegment returned error: %v", err)
	}

	results := Search(WrapSegment(m), "sunscreen", DefaultParams)
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2 (one from each segment)", len(results))
	}
}

func TestFilterDeleted_WorksAcrossMultiSegment(t *testing.T) {
	seg1Path := filepath.Join(t.TempDir(), "seg1.idx")
	p1 := extract.Passage{Text: "sunscreen leaves white cast", SourceURL: "https://a.com/1"}
	if err := diskindex.BuildSegment([]extract.Passage{p1}, seg1Path); err != nil {
		t.Fatalf("BuildSegment(seg1) returned error: %v", err)
	}

	seg2Path := filepath.Join(t.TempDir(), "seg2.idx")
	p2 := extract.Passage{Text: "sunscreen does not oxidize", SourceURL: "https://a.com/2"}
	if err := diskindex.BuildSegment([]extract.Passage{p2}, seg2Path); err != nil {
		t.Fatalf("BuildSegment(seg2) returned error: %v", err)
	}

	m, err := diskindex.OpenMultiSegment([]string{seg1Path, seg2Path})
	if err != nil {
		t.Fatalf("OpenMultiSegment returned error: %v", err)
	}

	logPath := filepath.Join(t.TempDir(), "deletions.jsonl")
	log, err := crawlstate.OpenDeletionLog(logPath)
	if err != nil {
		t.Fatalf("OpenDeletionLog returned error: %v", err)
	}
	defer log.Close()

	// Delete the passage that lives in the *second* segment, to prove
	// resolution works correctly at a non-zero segment offset too.
	if err := log.Delete(p2.ID(), "test deletion"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}

	results := Search(WrapSegment(m), "sunscreen", DefaultParams)
	if len(results) != 2 {
		t.Fatalf("got %d results before filtering, want 2", len(results))
	}

	filtered := FilterDeleted(results, m, log)
	if len(filtered) != 1 {
		t.Fatalf("got %d results after filtering, want 1", len(filtered))
	}
	if m.PassageID(filtered[0].DocID) != p1.ID() {
		t.Error("the remaining result should be p1, not the deleted p2")
	}
}

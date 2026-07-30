package bm25

import (
	"os"
	"path/filepath"
	"testing"

	"triedandtold/internal/crawlstate"
	"triedandtold/internal/diskindex"
	"triedandtold/internal/extract"
)

func buildSegmentAndDeletionLog(t *testing.T, passages []extract.Passage) (*diskindex.Segment, *crawlstate.DeletionLog) {
	t.Helper()

	segPath := filepath.Join(t.TempDir(), "segment.idx")
	if err := diskindex.BuildSegment(passages, segPath); err != nil {
		t.Fatalf("BuildSegment returned error: %v", err)
	}
	seg, err := diskindex.OpenSegment(segPath)
	if err != nil {
		t.Fatalf("OpenSegment returned error: %v", err)
	}

	logPath := filepath.Join(t.TempDir(), "deletions.jsonl")
	log, err := crawlstate.OpenDeletionLog(logPath)
	if err != nil {
		t.Fatalf("OpenDeletionLog returned error: %v", err)
	}
	t.Cleanup(func() { log.Close() })

	return seg, log
}

func TestFilterDeleted_ExcludesDeletedPassage(t *testing.T) {
	passages := []extract.Passage{
		{Text: "sunscreen leaves white cast", SourceURL: "https://a.com/1"},
		{Text: "sunscreen does not leave residue", SourceURL: "https://a.com/2"},
	}
	seg, log := buildSegmentAndDeletionLog(t, passages)

	if err := log.Delete(passages[0].ID(), "test deletion"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}

	results := Search(WrapSegment(seg), "sunscreen", DefaultParams)
	if len(results) != 2 {
		t.Fatalf("got %d results before filtering, want 2", len(results))
	}

	filtered := FilterDeleted(results, seg, log)
	if len(filtered) != 1 {
		t.Fatalf("got %d results after filtering, want 1", len(filtered))
	}
	if seg.PassageID(filtered[0].DocID) != passages[1].ID() {
		t.Errorf("remaining result is the wrong passage: got %q, want passage 1's ID", seg.PassageID(filtered[0].DocID))
	}
}

func TestFilterDeleted_NoDeletionsPreservesAllResultsInOrder(t *testing.T) {
	passages := []extract.Passage{
		{Text: "sunscreen white cast", SourceURL: "https://a.com/1"},
		{Text: "sunscreen only", SourceURL: "https://a.com/2"},
	}
	seg, log := buildSegmentAndDeletionLog(t, passages)

	results := Search(WrapSegment(seg), "sunscreen", DefaultParams)
	filtered := FilterDeleted(results, seg, log)

	if len(filtered) != len(results) {
		t.Fatalf("got %d results, want %d (no deletions, nothing should be filtered)", len(filtered), len(results))
	}
	for i := range results {
		if filtered[i] != results[i] {
			t.Errorf("result %d changed: got %+v, want %+v (order/content must be preserved)", i, filtered[i], results[i])
		}
	}
}

func TestFilterDeleted_AllDeletedReturnsEmpty(t *testing.T) {
	passages := []extract.Passage{
		{Text: "sunscreen white cast", SourceURL: "https://a.com/1"},
	}
	seg, log := buildSegmentAndDeletionLog(t, passages)
	log.Delete(passages[0].ID(), "test deletion")

	results := Search(WrapSegment(seg), "sunscreen", DefaultParams)
	filtered := FilterDeleted(results, seg, log)

	if len(filtered) != 0 {
		t.Errorf("got %d results, want 0 (everything was deleted)", len(filtered))
	}
}

// TestFilterDeleted_RealExtractedPassages proves the wiring end-to-end:
// real extractor output, a real disk segment, a real deletion log, and a
// real search - a deleted real passage must not appear in results.
func TestFilterDeleted_RealExtractedPassages(t *testing.T) {
	html, err := os.ReadFile("../extract/testdata/example_site.html")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}
	passages, err := extract.ExampleSiteExtractor{}.Extract(string(html), "https://example-reviews.test/product/1")
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}

	seg, log := buildSegmentAndDeletionLog(t, passages)

	results := Search(WrapSegment(seg), "sunscreen white cast", DefaultParams)
	if len(results) == 0 {
		t.Fatal("expected at least one result before deletion")
	}
	toDelete := results[0]
	deletedPassageID := seg.PassageID(toDelete.DocID)
	if err := log.Delete(deletedPassageID, "test deletion"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}

	filtered := FilterDeleted(results, seg, log)
	for _, r := range filtered {
		if seg.PassageID(r.DocID) == deletedPassageID {
			t.Errorf("deleted passage %q still present in filtered results", deletedPassageID)
		}
	}
	if len(filtered) != len(results)-1 {
		t.Errorf("got %d filtered results, want %d (exactly one removed)", len(filtered), len(results)-1)
	}
}

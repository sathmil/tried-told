package crawlstate

import (
	"os"
	"path/filepath"
	"testing"

	"triedandtold/internal/extract"
)

// TestDeletionLog_ExcludesRealDeletedPassageFromRealExtractedSet proves the
// wiring end-to-end: extract real passages, delete one by its real
// content-based ID, and confirm exactly that one - and only that one - is
// reported deleted among the whole real set.
func TestDeletionLog_ExcludesRealDeletedPassageFromRealExtractedSet(t *testing.T) {
	html, err := os.ReadFile("../extract/testdata/example_site.html")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	passages, err := extract.ExampleSiteExtractor{}.Extract(string(html), "https://example-reviews.test/product/1")
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}
	if len(passages) < 2 {
		t.Fatalf("got %d passages, want at least 2 to make this test meaningful", len(passages))
	}

	path := filepath.Join(t.TempDir(), "deletions.jsonl")
	d, err := OpenDeletionLog(path)
	if err != nil {
		t.Fatalf("OpenDeletionLog returned error: %v", err)
	}
	defer d.Close()

	toDelete := passages[0]
	if err := d.Delete(toDelete.ID(), "test deletion"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}

	var remaining []extract.Passage
	for _, p := range passages {
		if !d.IsDeleted(p.ID()) {
			remaining = append(remaining, p)
		}
	}

	if len(remaining) != len(passages)-1 {
		t.Fatalf("got %d remaining passages, want %d", len(remaining), len(passages)-1)
	}
	for _, p := range remaining {
		if p.ID() == toDelete.ID() {
			t.Errorf("deleted passage %+v was still present in the remaining set", p)
		}
	}
}

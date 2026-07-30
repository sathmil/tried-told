package diskindex

import (
	"os"
	"path/filepath"
	"testing"

	"triedandtold/internal/extract"
)

func TestBuildAndOpenSegment_RoundTrip(t *testing.T) {
	passages := []extract.Passage{
		{Text: "sunscreen leaves white cast", SourceURL: "https://a.com/1"},
		{Text: "sunscreen does not leave residue", SourceURL: "https://a.com/2"},
		{Text: "moisturizer is great", SourceURL: "https://a.com/3"},
	}

	path := filepath.Join(t.TempDir(), "segment.idx")
	if err := BuildSegment(passages, path); err != nil {
		t.Fatalf("BuildSegment returned error: %v", err)
	}

	seg, err := OpenSegment(path)
	if err != nil {
		t.Fatalf("OpenSegment returned error: %v", err)
	}

	if seg.N() != 3 {
		t.Errorf("N() = %d, want 3", seg.N())
	}
	if got := seg.DocLen(0); got != 4 { // "sunscreen leaves white cast" = 4 tokens
		t.Errorf("DocLen(0) = %d, want 4", got)
	}

	for i, p := range passages {
		if got := seg.PassageID(i); got != p.ID() {
			t.Errorf("PassageID(%d) = %q, want %q", i, got, p.ID())
		}
	}

	if df := seg.DocFreq("sunscreen"); df != 2 {
		t.Errorf("DocFreq(sunscreen) = %d, want 2", df)
	}

	postingsList, ok := seg.TermPostings("sunscreen")
	if !ok {
		t.Fatal("TermPostings(sunscreen) not found")
	}
	if len(postingsList) != 2 {
		t.Fatalf("got %d postings, want 2", len(postingsList))
	}
	if postingsList[0].LocalDocID != 0 || postingsList[1].LocalDocID != 1 {
		t.Errorf("postings docIDs = [%d, %d], want [0, 1]", postingsList[0].LocalDocID, postingsList[1].LocalDocID)
	}
	if len(postingsList[0].Positions) != 1 || postingsList[0].Positions[0] != 0 {
		t.Errorf("doc0 positions = %v, want [0] (sunscreen is the first token)", postingsList[0].Positions)
	}

	if _, ok := seg.TermPostings("nonexistent"); ok {
		t.Error("TermPostings(nonexistent) returned ok=true")
	}
}

func TestBuildAndOpenSegment_Empty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.idx")
	if err := BuildSegment(nil, path); err != nil {
		t.Fatalf("BuildSegment(nil) returned error: %v", err)
	}

	seg, err := OpenSegment(path)
	if err != nil {
		t.Fatalf("OpenSegment returned error: %v", err)
	}
	if seg.N() != 0 {
		t.Errorf("N() = %d, want 0", seg.N())
	}
	if _, ok := seg.TermPostings("anything"); ok {
		t.Error("TermPostings on an empty segment returned ok=true")
	}
}

// TestOpenSegment_CorruptionIsDetected is the "integrity checks and
// corruption tests" requirement made concrete at the segment level: a
// single flipped bit anywhere in the file must be caught by the checksum,
// not silently produce wrong search results.
func TestOpenSegment_CorruptionIsDetected(t *testing.T) {
	passages := []extract.Passage{
		{Text: "sunscreen leaves white cast", SourceURL: "https://a.com/1"},
	}
	path := filepath.Join(t.TempDir(), "segment.idx")
	if err := BuildSegment(passages, path); err != nil {
		t.Fatalf("BuildSegment returned error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read segment file: %v", err)
	}
	corruptAt := len(data) / 2
	data[corruptAt] ^= 0xFF
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("failed to write corrupted file: %v", err)
	}

	if _, err := OpenSegment(path); err == nil {
		t.Error("OpenSegment on a corrupted file returned no error")
	}
}

// TestBuildSegment_RealExtractedPassages proves the wiring end-to-end:
// real passages from the real extractor, built into a real segment file,
// read back correctly - not a synthetic example built solely for this
// package's own tests.
func TestBuildSegment_RealExtractedPassages(t *testing.T) {
	html, err := os.ReadFile("../extract/testdata/example_site.html")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}
	passages, err := extract.ExampleSiteExtractor{}.Extract(string(html), "https://example-reviews.test/product/1")
	if err != nil {
		t.Fatalf("Extract returned error: %v", err)
	}

	path := filepath.Join(t.TempDir(), "segment.idx")
	if err := BuildSegment(passages, path); err != nil {
		t.Fatalf("BuildSegment returned error: %v", err)
	}

	seg, err := OpenSegment(path)
	if err != nil {
		t.Fatalf("OpenSegment returned error: %v", err)
	}

	if seg.N() != len(passages) {
		t.Fatalf("N() = %d, want %d", seg.N(), len(passages))
	}
	if _, ok := seg.TermPostings("sunscreen"); !ok {
		t.Error(`TermPostings("sunscreen") not found in real extracted data`)
	}
	for i, p := range passages {
		if got := seg.PassageID(i); got != p.ID() {
			t.Errorf("PassageID(%d) = %q, want %q", i, got, p.ID())
		}
	}
}

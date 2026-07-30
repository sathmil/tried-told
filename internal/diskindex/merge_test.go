package diskindex

import (
	"path/filepath"
	"testing"

	"triedandtold/internal/crawlstate"
	"triedandtold/internal/extract"
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

func buildSegment(t *testing.T, passages []extract.Passage) *Segment {
	t.Helper()
	path := filepath.Join(t.TempDir(), "segment.idx")
	if err := BuildSegment(passages, path); err != nil {
		t.Fatalf("BuildSegment returned error: %v", err)
	}
	seg, err := OpenSegment(path)
	if err != nil {
		t.Fatalf("OpenSegment returned error: %v", err)
	}
	return seg
}

func TestMergeSegments_CombinesPostingsFromBothSegments(t *testing.T) {
	a := []extract.Passage{
		{Text: "sunscreen leaves white cast", SourceURL: "https://a.com/1"},
		{Text: "sunscreen absorbs fast", SourceURL: "https://a.com/2"},
	}
	b := []extract.Passage{
		{Text: "moisturizer is great", SourceURL: "https://b.com/1"},
		{Text: "sunscreen and moisturizer both work", SourceURL: "https://b.com/2"},
	}
	segA, segB := buildSegment(t, a), buildSegment(t, b)

	mergedPath := filepath.Join(t.TempDir(), "merged.idx")
	log := openDeletionLog(t)
	if err := MergeSegments([]*Segment{segA, segB}, log, mergedPath); err != nil {
		t.Fatalf("MergeSegments returned error: %v", err)
	}
	merged, err := OpenSegment(mergedPath)
	if err != nil {
		t.Fatalf("OpenSegment(merged) returned error: %v", err)
	}

	if merged.N() != 4 {
		t.Fatalf("N() = %d, want 4 (2 from each segment)", merged.N())
	}
	if merged.DocFreq("sunscreen") != 3 {
		t.Errorf(`DocFreq("sunscreen") = %d, want 3 (2 from segment A, 1 from segment B)`, merged.DocFreq("sunscreen"))
	}

	// Every original passage ID must be present exactly once, regardless
	// of which input segment it came from.
	want := map[string]bool{}
	for _, p := range append(append([]extract.Passage{}, a...), b...) {
		want[p.ID()] = true
	}
	for i := 0; i < merged.N(); i++ {
		pid := merged.PassageID(i)
		if !want[pid] {
			t.Errorf("PassageID(%d) = %q, not one of the original passages", i, pid)
		}
		delete(want, pid)
	}
	if len(want) != 0 {
		t.Errorf("%d original passages missing from the merged segment", len(want))
	}
}

// TestMergeSegments_DropsTombstonedPassages is the proof that actually
// matters: merging must reclaim space for deleted passages, not just
// continue to hide them the way query-time filtering already does.
func TestMergeSegments_DropsTombstonedPassages(t *testing.T) {
	a := []extract.Passage{
		{Text: "sunscreen leaves white cast", SourceURL: "https://a.com/1"},
		{Text: "moisturizer is great", SourceURL: "https://a.com/2"},
	}
	b := []extract.Passage{
		{Text: "unobtaniumcream fixes everything", SourceURL: "https://b.com/1"},
	}
	segA, segB := buildSegment(t, a), buildSegment(t, b)

	log := openDeletionLog(t)
	if err := log.Delete(a[0].ID(), "test deletion"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if err := log.Delete(b[0].ID(), "test deletion"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}

	mergedPath := filepath.Join(t.TempDir(), "merged.idx")
	if err := MergeSegments([]*Segment{segA, segB}, log, mergedPath); err != nil {
		t.Fatalf("MergeSegments returned error: %v", err)
	}
	merged, err := OpenSegment(mergedPath)
	if err != nil {
		t.Fatalf("OpenSegment(merged) returned error: %v", err)
	}

	if merged.N() != 1 {
		t.Fatalf("N() = %d, want 1 (only the moisturizer passage survives)", merged.N())
	}
	if merged.PassageID(0) != a[1].ID() {
		t.Errorf("PassageID(0) = %q, want the moisturizer passage's ID", merged.PassageID(0))
	}

	// "unobtaniumcream" appeared only in the deleted passage - it must
	// not merely be unsearchable, it must be entirely gone from the
	// merged dictionary, which is what actually reclaims the space.
	for _, term := range merged.Terms() {
		if term == "unobtaniumcream" {
			t.Errorf("term %q from a deleted passage survived the merge", term)
		}
	}
	if merged.DocFreq("unobtaniumcream") != 0 {
		t.Errorf(`DocFreq("unobtaniumcream") = %d, want 0`, merged.DocFreq("unobtaniumcream"))
	}
}

// TestMergeSegments_PreservesPositionsForPhraseSearch proves positions
// survive renumbering intact, not just doc-level frequencies.
func TestMergeSegments_PreservesPositionsForPhraseSearch(t *testing.T) {
	a := []extract.Passage{
		{Text: "this sunscreen leaves a white cast on my skin", SourceURL: "https://a.com/1"},
	}
	b := []extract.Passage{
		{Text: "no white cast at all with this one", SourceURL: "https://b.com/1"},
	}
	segA, segB := buildSegment(t, a), buildSegment(t, b)

	mergedPath := filepath.Join(t.TempDir(), "merged.idx")
	log := openDeletionLog(t)
	if err := MergeSegments([]*Segment{segA, segB}, log, mergedPath); err != nil {
		t.Fatalf("MergeSegments returned error: %v", err)
	}
	merged, err := OpenSegment(mergedPath)
	if err != nil {
		t.Fatalf("OpenSegment(merged) returned error: %v", err)
	}

	hits := merged.PhraseSearch("white cast")
	if len(hits) != 2 {
		t.Fatalf("PhraseSearch(\"white cast\") = %v, want both passages", hits)
	}
}

// TestMergeSegments_ThreeWayMergeSumsAllSegments confirms merging isn't
// limited to pairs - it takes any number of segments in one pass.
func TestMergeSegments_ThreeWayMergeSumsAllSegments(t *testing.T) {
	segs := []*Segment{
		buildSegment(t, []extract.Passage{{Text: "alpha", SourceURL: "https://x.com/1"}}),
		buildSegment(t, []extract.Passage{{Text: "beta", SourceURL: "https://x.com/2"}}),
		buildSegment(t, []extract.Passage{{Text: "gamma", SourceURL: "https://x.com/3"}}),
	}

	mergedPath := filepath.Join(t.TempDir(), "merged.idx")
	log := openDeletionLog(t)
	if err := MergeSegments(segs, log, mergedPath); err != nil {
		t.Fatalf("MergeSegments returned error: %v", err)
	}
	merged, err := OpenSegment(mergedPath)
	if err != nil {
		t.Fatalf("OpenSegment(merged) returned error: %v", err)
	}

	if merged.N() != 3 {
		t.Fatalf("N() = %d, want 3", merged.N())
	}
	for _, term := range []string{"alpha", "beta", "gamma"} {
		if merged.DocFreq(term) != 1 {
			t.Errorf("DocFreq(%q) = %d, want 1", term, merged.DocFreq(term))
		}
	}
}

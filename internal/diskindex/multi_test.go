package diskindex

import (
	"path/filepath"
	"testing"

	"triedandtold/internal/extract"
)

func buildAndOpen(t *testing.T, passages []extract.Passage, name string) *Segment {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := BuildSegment(passages, path); err != nil {
		t.Fatalf("BuildSegment(%s) returned error: %v", name, err)
	}
	seg, err := OpenSegment(path)
	if err != nil {
		t.Fatalf("OpenSegment(%s) returned error: %v", name, err)
	}
	return seg
}

func TestMultiSegment_CombinesTwoSegmentsIntoOneIDSpace(t *testing.T) {
	seg1 := buildAndOpen(t, []extract.Passage{
		{Text: "sunscreen leaves white cast", SourceURL: "https://a.com/1"},
		{Text: "moisturizer is great", SourceURL: "https://a.com/2"},
	}, "seg1.idx")
	seg2 := buildAndOpen(t, []extract.Passage{
		{Text: "sunscreen does not oxidize", SourceURL: "https://a.com/3"},
	}, "seg2.idx")

	m := NewMultiSegment([]*Segment{seg1, seg2})

	if m.N() != 3 {
		t.Errorf("N() = %d, want 3", m.N())
	}

	// "sunscreen" appears in seg1's local doc 0 and seg2's local doc 0,
	// which must become distinct global IDs 0 and 2.
	postingsList, ok := m.TermPostings("sunscreen")
	if !ok {
		t.Fatal("TermPostings(sunscreen) not found")
	}
	if len(postingsList) != 2 {
		t.Fatalf("got %d postings, want 2", len(postingsList))
	}
	gotIDs := map[int]bool{postingsList[0].LocalDocID: true, postingsList[1].LocalDocID: true}
	if !gotIDs[0] || !gotIDs[2] {
		t.Errorf("postings global DocIDs = %v, want {0, 2}", gotIDs)
	}

	// Global ID 2 (seg2's local doc 0) must resolve through seg2, not seg1.
	seg2Passage := extract.Passage{Text: "sunscreen does not oxidize", SourceURL: "https://a.com/3"}
	if got, want := m.PassageID(2), seg2Passage.ID(); got != want {
		t.Errorf("PassageID(2) = %q, want %q", got, want)
	}
	if got, want := m.DocLen(2), 4; got != want { // "sunscreen does not oxidize" = 4 tokens
		t.Errorf("DocLen(2) = %d, want %d", got, want)
	}

	// "moisturizer" only appears in seg1's local doc 1 -> global ID 1.
	moistPostings, ok := m.TermPostings("moisturizer")
	if !ok || len(moistPostings) != 1 || moistPostings[0].LocalDocID != 1 {
		t.Errorf("TermPostings(moisturizer) = %v, ok=%v, want a single posting at global ID 1", moistPostings, ok)
	}
}

func TestMultiSegment_PhraseSearchAcrossSegments(t *testing.T) {
	seg1 := buildAndOpen(t, []extract.Passage{
		{Text: "this leaves a white cast on skin", SourceURL: "https://a.com/1"},
	}, "seg1.idx")
	seg2 := buildAndOpen(t, []extract.Passage{
		{Text: "the cast wore white shirts", SourceURL: "https://a.com/2"}, // words present, not a phrase match
		{Text: "another white cast complaint", SourceURL: "https://a.com/3"},
	}, "seg2.idx")

	m := NewMultiSegment([]*Segment{seg1, seg2})

	matches := m.PhraseSearch("white cast")
	want := map[int]bool{0: true, 2: true} // global 0 (seg1 doc0), global 2 (seg2 doc1)
	if len(matches) != len(want) {
		t.Fatalf("PhraseSearch(\"white cast\") = %v, want matches at global IDs 0 and 2", matches)
	}
	for _, m := range matches {
		if !want[m] {
			t.Errorf("unexpected match at global ID %d", m)
		}
	}
}

func TestMultiSegment_EmptyConstituentSegment(t *testing.T) {
	seg1 := buildAndOpen(t, nil, "empty.idx")
	seg2 := buildAndOpen(t, []extract.Passage{
		{Text: "sunscreen is great", SourceURL: "https://a.com/1"},
	}, "seg2.idx")

	m := NewMultiSegment([]*Segment{seg1, seg2})
	if m.N() != 1 {
		t.Errorf("N() = %d, want 1", m.N())
	}
	if _, ok := m.TermPostings("sunscreen"); !ok {
		t.Error("TermPostings(sunscreen) not found across an empty + non-empty segment")
	}
}

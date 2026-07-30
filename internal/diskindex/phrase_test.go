package diskindex

import (
	"path/filepath"
	"testing"

	"triedandtold/internal/extract"
)

func buildTestSegment(t *testing.T, texts []string) *Segment {
	t.Helper()
	passages := make([]extract.Passage, len(texts))
	for i, text := range texts {
		passages[i] = extract.Passage{Text: text, SourceURL: "https://a.com/1"}
	}
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

// TestPhraseSearch_RequiresAdjacentInOrderWords is the core proof that
// positions are doing real work here, not just an AND query in disguise:
// a document containing both words but not adjacent (or in reverse order)
// must NOT match, while one containing them as an actual consecutive
// phrase must.
func TestPhraseSearch_RequiresAdjacentInOrderWords(t *testing.T) {
	seg := buildTestSegment(t, []string{
		"the cast wore white shirts today",     // doc 0: has both words, not adjacent, reverse order
		"this leaves a white cast on skin",     // doc 1: "white cast" as an exact consecutive phrase
		"no relevant words in this one at all", // doc 2: neither word
	})

	matches := seg.PhraseSearch("white cast")
	if len(matches) != 1 || matches[0] != 1 {
		t.Errorf("PhraseSearch(\"white cast\") = %v, want [1]", matches)
	}
}

func TestPhraseSearch_ThreeWordPhrase(t *testing.T) {
	seg := buildTestSegment(t, []string{
		"sunscreen does not leave residue",      // doc 0: exact match
		"sunscreen does not ever leave residue", // doc 1: extra word breaks the phrase
		"leave does not sunscreen",              // doc 2: same words, wrong order
	})

	matches := seg.PhraseSearch("does not leave")
	if len(matches) != 1 || matches[0] != 0 {
		t.Errorf(`PhraseSearch("does not leave") = %v, want [0]`, matches)
	}
}

func TestPhraseSearch_SingleWordDegeneratesToTermLookup(t *testing.T) {
	seg := buildTestSegment(t, []string{
		"sunscreen is great",
		"moisturizer is great",
	})

	matches := seg.PhraseSearch("sunscreen")
	if len(matches) != 1 || matches[0] != 0 {
		t.Errorf(`PhraseSearch("sunscreen") = %v, want [0]`, matches)
	}
}

func TestPhraseSearch_MissingTermReturnsNoMatches(t *testing.T) {
	seg := buildTestSegment(t, []string{"sunscreen is great"})

	matches := seg.PhraseSearch("sunscreen amazing")
	if len(matches) != 0 {
		t.Errorf(`PhraseSearch("sunscreen amazing") = %v, want empty (word "amazing" never appears)`, matches)
	}
}

func TestPhraseSearch_MultipleMatchesAcrossDocs(t *testing.T) {
	seg := buildTestSegment(t, []string{
		"white cast on my deep skin",
		"another white cast complaint here",
		"nothing relevant in this one",
	})

	matches := seg.PhraseSearch("white cast")
	want := map[int]bool{0: true, 1: true}
	if len(matches) != len(want) {
		t.Fatalf("PhraseSearch(\"white cast\") = %v, want matches for docs 0 and 1", matches)
	}
	for _, m := range matches {
		if !want[m] {
			t.Errorf("unexpected match doc %d", m)
		}
	}
}

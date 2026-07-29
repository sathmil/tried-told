package bm25

import (
	"math"
	"testing"

	"triedandtold/internal/index"
)

func TestIDF_RarerTermScoresHigher(t *testing.T) {
	rare := idf(5, 1)   // term in 1 of 5 docs
	common := idf(5, 5) // term in all 5 docs
	if rare <= common {
		t.Errorf("idf(rare)=%v, idf(common)=%v; want rare strictly higher", rare, common)
	}
}

func TestIDF_NeverNegative(t *testing.T) {
	// A term in every document must still score >= 0: we rely on IDF to
	// self-suppress common words instead of removing stopwords (design doc 02).
	if got := idf(100, 100); got < 0 {
		t.Errorf("idf(100,100) = %v, want >= 0", got)
	}
}

func TestTermScore_SaturatesAtK1Plus1(t *testing.T) {
	p := Params{K1: 1.5, B: 0.75}
	got := termScore(1_000_000_000, 10, 10, p) // docLen == avgDocLen isolates the freq effect
	want := p.K1 + 1
	if math.Abs(got-want) > 1e-6 {
		t.Errorf("termScore with huge freq = %v, want ~%v (k1+1)", got, want)
	}
	if got >= want {
		t.Errorf("termScore = %v, must stay strictly below k1+1 = %v", got, want)
	}
}

func TestSearch_ExcludesNonMatchingDocs(t *testing.T) {
	idx := index.BuildIndex([]index.IndexDoc{
		{ID: 0, Text: "sunscreen"},
		{ID: 1, Text: "moisturizer"},
	})
	results := Search(idx, "sunscreen", DefaultParams)
	if len(results) != 1 || results[0].DocID != 0 {
		t.Errorf("Search = %v, want exactly doc 0", results)
	}
}

func TestSearch_UnknownTermReturnsNoResults(t *testing.T) {
	idx := index.BuildIndex([]index.IndexDoc{{ID: 0, Text: "sunscreen"}})
	results := Search(idx, "nonexistentterm", DefaultParams)
	if len(results) != 0 {
		t.Errorf("Search = %v, want empty", results)
	}
}

func TestSearch_HigherFrequencyScoresHigher(t *testing.T) {
	idx := index.BuildIndex([]index.IndexDoc{
		{ID: 0, Text: "sunscreen filler filler"},   // freq(sunscreen)=1, len 3
		{ID: 1, Text: "sunscreen sunscreen filler"}, // freq(sunscreen)=2, len 3
	})
	results := Search(idx, "sunscreen", DefaultParams)
	if len(results) != 2 || results[0].DocID != 1 {
		t.Errorf("Search = %v, want doc 1 (freq 2) ranked above doc 0 (freq 1)", results)
	}
}

func TestSearch_MatchingMoreQueryTermsScoresHigher(t *testing.T) {
	idx := index.BuildIndex([]index.IndexDoc{
		{ID: 0, Text: "sunscreen white cast"},
		{ID: 1, Text: "sunscreen only"},
	})
	results := Search(idx, "sunscreen white cast", DefaultParams)
	if len(results) != 2 || results[0].DocID != 0 {
		t.Errorf("Search = %v, want doc 0 (matches all 3 terms) ranked above doc 1", results)
	}
}

func TestSearch_TiesBreakByAscendingDocID(t *testing.T) {
	idx := index.BuildIndex([]index.IndexDoc{
		{ID: 0, Text: "sunscreen"},
		{ID: 1, Text: "sunscreen"},
	})
	results := Search(idx, "sunscreen", DefaultParams)
	if len(results) != 2 || results[0].DocID != 0 || results[1].DocID != 1 {
		t.Errorf("Search = %v, want [doc0, doc1] in that order (tie broken by DocID)", results)
	}
}

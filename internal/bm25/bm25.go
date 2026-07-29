// Package bm25 implements Okapi BM25 scoring and search over an
// index.Index, per docs/design/04-bm25.md.
package bm25

import (
	"math"
	"sort"

	"triedandtold/internal/index"
	"triedandtold/internal/tokenize"
)

// Params holds BM25's tunable constants. k1 controls how quickly repeated
// term occurrences saturate; b controls how strongly document length is
// penalized (0 = no penalty, 1 = full proportional penalty).
type Params struct {
	K1 float64
	B  float64
}

// DefaultParams are the standard literature starting point.
var DefaultParams = Params{K1: 1.5, B: 0.75}

// Result is one scored document from a search.
type Result struct {
	DocID int
	Score float64
}

// idf is the smoothed inverse document frequency: always >= 0, so a term's
// presence never actively lowers a document's score, even for very common
// terms (relevant since we don't remove stopwords - see docs/design/02).
func idf(n, df int) float64 {
	return math.Log((float64(n-df)+0.5)/(float64(df)+0.5) + 1)
}

// termScore is the TF / length-normalization fraction for one posting.
func termScore(freq, docLen int, avgDocLen float64, p Params) float64 {
	num := float64(freq) * (p.K1 + 1)
	den := float64(freq) + p.K1*(1-p.B+p.B*float64(docLen)/avgDocLen)
	return num / den
}

// Search tokenizes query, scores every document that shares at least one
// term with it, and returns results sorted by score descending (ties broken
// by ascending DocID, for deterministic output).
func Search(idx index.Index, query string, p Params) []Result {
	scores := make(map[int]float64)

	for _, term := range tokenize.Tokenize(query) {
		postings, ok := idx.Postings[term]
		if !ok {
			continue
		}
		weight := idf(idx.N, len(postings))
		for _, posting := range postings {
			scores[posting.DocID] += weight * termScore(posting.Freq, idx.DocLen[posting.DocID], idx.AvgDocLen, p)
		}
	}

	results := make([]Result, 0, len(scores))
	for docID, score := range scores {
		results = append(results, Result{DocID: docID, Score: score})
	}
	sort.Slice(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		return results[i].DocID < results[j].DocID
	})

	return results
}

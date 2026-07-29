package index

import (
	"fmt"

	"triedandtold/internal/tokenize"
)

// BuildIndex builds an in-memory inverted index from docs.
//
// docs must be contiguously numbered 0, 1, 2, ... in order. BuildIndex
// panics if that invariant is violated: postings lists rely on it to stay
// sorted by DocID without an explicit sort step, and DocLen relies on it to
// use DocID directly as a slice index.
func BuildIndex(docs []IndexDoc) Index {
	postings := make(map[string][]Posting)
	var docLen []int
	totalLen := 0
	lastID := -1

	for _, doc := range docs {
		if doc.ID != lastID+1 {
			panic(fmt.Sprintf("index.BuildIndex: documents must be contiguously numbered, got ID %d after %d", doc.ID, lastID))
		}
		lastID = doc.ID

		tokens := tokenize.Tokenize(doc.Text)
		docLen = append(docLen, len(tokens))
		totalLen += len(tokens)

		termFreq := make(map[string]int)
		for _, tok := range tokens {
			termFreq[tok]++
		}
		for term, freq := range termFreq {
			postings[term] = append(postings[term], Posting{DocID: doc.ID, Freq: freq})
		}
	}

	avgDocLen := 0.0
	if len(docs) > 0 {
		avgDocLen = float64(totalLen) / float64(len(docs))
	}

	return Index{
		Postings:  postings,
		DocLen:    docLen,
		AvgDocLen: avgDocLen,
		N:         len(docs),
	}
}

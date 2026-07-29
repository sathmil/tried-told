package index

import (
	"fmt"

	"triedandtold/internal/tokenize"
)

// BuildIndex builds an in-memory inverted index from docs.
//
// docs must be sorted strictly ascending by ID. BuildIndex panics if that
// invariant is violated, since every postings list it produces relies on
// staying sorted by DocID without ever running an explicit sort step.
func BuildIndex(docs []IndexDoc) Index {
	idx := make(Index)
	lastID := -1

	for _, doc := range docs {
		if doc.ID <= lastID {
			panic(fmt.Sprintf("index.BuildIndex: documents must be strictly ascending by ID, got %d after %d", doc.ID, lastID))
		}
		lastID = doc.ID

		termFreq := make(map[string]int)
		for _, tok := range tokenize.Tokenize(doc.Text) {
			termFreq[tok]++
		}

		for term, freq := range termFreq {
			idx[term] = append(idx[term], Posting{DocID: doc.ID, Freq: freq})
		}
	}

	return idx
}

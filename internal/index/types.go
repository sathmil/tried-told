// Package index implements the in-memory inverted index, per
// docs/design/01-document-representation-and-index.md.
package index

// IndexDoc holds what the tokenizer/index needs to touch.
type IndexDoc struct {
	ID   int
	Text string
}

// DocMeta holds what search results need to render. It shares the same ID
// as the corresponding IndexDoc.
type DocMeta struct {
	ID       int
	Source   string
	Product  string
	SkinTone string // as explicitly self-described in the text; empty if unstated
	Climate  string // as explicitly self-described in the text; empty if unstated
}

// Posting records that a term occurred Freq times in document DocID.
type Posting struct {
	DocID int
	Freq  int
}

// Index holds the postings map plus the per-corpus statistics BM25 needs
// (document length is a property of the document, not of any term, so it's
// stored once per DocID rather than duplicated into every posting).
type Index struct {
	Postings  map[string][]Posting // term -> postings, sorted ascending by DocID
	DocLen    []int                // DocLen[docID] = token count of that document
	AvgDocLen float64
	N         int // total number of documents
}

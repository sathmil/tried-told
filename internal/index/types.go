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

// Index maps a term to its postings list, sorted ascending by DocID.
type Index map[string][]Posting

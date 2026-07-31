// Package semantic wraps an HNSW approximate-nearest-neighbor index
// (github.com/coder/hnsw) over passage embeddings, keyed by
// extract.Passage.ID(). See docs/design/26-hnsw-index.md.
package semantic

import (
	"github.com/coder/hnsw"

	"triedandtold/internal/embeddings"
)

// Index is a persistent HNSW graph over passage embeddings.
type Index struct {
	graph *hnsw.SavedGraph[string]
}

// Open loads the HNSW index at path, or creates a new empty one if path
// doesn't exist yet (LoadSavedGraph's documented behavior - the same file
// path is used for first creation and every later reopen). Distance is
// cosine, matching bge-small-en-v1.5's normalized-embedding convention.
func Open(path string) (*Index, error) {
	g, err := hnsw.LoadSavedGraph[string](path)
	if err != nil {
		return nil, err
	}
	g.Distance = hnsw.CosineDistance
	return &Index{graph: g}, nil
}

// Add inserts every embedding in f into the index and persists the
// result. Re-adding a passage ID that's already present replaces its
// vector, so Add is also how an updated embedding gets applied - there's
// no separate update operation.
//
// The underlying library's own docs claim Add already replaces an
// existing key automatically ("If another node with the same ID exists,
// it is replaced") - verified directly and found false for v0.6.1: Add
// panics ("node not added") if the key already exists. The correct
// sequence is Delete then Add, which works - except deleting the *only*
// remaining node leaves the graph in a broken state where the next Add
// panics with a nil pointer dereference. Both are library bugs, not
// something to route around unnoticed - see docs/design/26-hnsw-index.md.
func (idx *Index) Add(f *embeddings.File) error {
	// A real crawl surfaced a case the graph-state check above doesn't
	// cover: f itself can contain two embeddings with the same
	// PassageID (e.g. a decorative element repeated verbatim within one
	// page - identical SourceURL+Text hashes identically). Neither
	// exists in the graph yet, so both would pass the existing-key
	// check below and get queued into the same Add call - hitting the
	// exact "duplicate key" library bug this function otherwise routes
	// around, just within one batch instead of across two. Deduping
	// first, last-occurrence-wins (consistent with "re-adding a passage
	// ID replaces its vector"), closes that gap.
	deduped := make(map[string]embeddings.Embedding, len(f.Embeddings))
	order := make([]string, 0, len(f.Embeddings))
	for _, e := range f.Embeddings {
		if _, seen := deduped[e.PassageID]; !seen {
			order = append(order, e.PassageID)
		}
		deduped[e.PassageID] = e
	}

	for _, id := range order {
		if _, exists := idx.graph.Lookup(id); exists {
			if idx.graph.Len() == 1 {
				// Deleting the sole remaining node breaks the graph (see
				// above) - recreating it fresh sidesteps that entirely,
				// verified empirically to work correctly.
				idx.graph.Graph = hnsw.NewGraph[string]()
				idx.graph.Distance = hnsw.CosineDistance
			} else {
				idx.graph.Delete(id)
			}
		}
	}

	nodes := make([]hnsw.Node[string], len(order))
	for i, id := range order {
		nodes[i] = hnsw.MakeNode(id, deduped[id].Vector)
	}
	idx.graph.Add(nodes...)
	return idx.graph.Save()
}

// Search returns the k passage IDs whose embeddings are nearest to query
// (by cosine distance), ranked closest-first. This is an approximate
// search - HNSW trades a small, tunable chance of missing the true
// nearest neighbors for far faster query time than exact search.
func (idx *Index) Search(query []float32, k int) []string {
	neighbors := idx.graph.Search(query, k)
	out := make([]string, len(neighbors))
	for i, n := range neighbors {
		out[i] = n.Key
	}
	return out
}

// Len returns the number of passages currently in the index.
func (idx *Index) Len() int { return idx.graph.Len() }

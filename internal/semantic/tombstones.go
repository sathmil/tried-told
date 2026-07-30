package semantic

// Deleter reports whether a passage (identified by its stable
// extract.Passage.ID()) has been deleted. *crawlstate.DeletionLog
// satisfies this directly. Defined locally rather than reusing
// bm25.Deleter (an identically-shaped interface) so this package
// doesn't need to import bm25 just to name a one-method interface.
type Deleter interface {
	IsDeleted(passageID string) bool
}

// FilterDeleted removes any tombstoned passage ID from results,
// preserving order. Simpler than bm25.FilterDeleted: Search already
// returns stable Passage IDs directly, so there's no local DocID to
// resolve first.
func FilterDeleted(results []string, deleted Deleter) []string {
	out := make([]string, 0, len(results))
	for _, id := range results {
		if deleted.IsDeleted(id) {
			continue
		}
		out = append(out, id)
	}
	return out
}

// overFetchMultiplier is a heuristic, not a literature-backed constant
// like BM25's k1/b or RRF's k=60: querying for more neighbors than
// asked for gives FilterDeleted room to drop tombstoned candidates
// without necessarily shrinking below k results. 3x is a starting point
// to revisit against real deletion rates, not a tuned value.
const overFetchMultiplier = 3

// SearchLive is Search, but tombstone-aware: it asks the HNSW graph for
// up to k*overFetchMultiplier candidates, filters out anything deleted,
// and returns at most k of what's left.
//
// This can still return fewer than k if enough of the over-fetched
// candidates are deleted - over-fetching narrows that gap, it doesn't
// close it. That's the tradeoff accepted over mutating the graph
// directly (deleting tombstoned nodes so Search never sees them at
// all), which would avoid the gap entirely but reopen the coder/hnsw
// deletion bug from docs/design/26-hnsw-index.md for routine use
// instead of a one-off replace. See docs/design/30-semantic-tombstones.md.
func (idx *Index) SearchLive(query []float32, k int, deleted Deleter) []string {
	candidates := idx.Search(query, k*overFetchMultiplier)
	live := FilterDeleted(candidates, deleted)
	if len(live) > k {
		live = live[:k]
	}
	return live
}

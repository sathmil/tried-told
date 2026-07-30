// Package hybrid combines multiple independent rankings of the same
// passages - e.g. BM25's lexical ranking and the HNSW index's semantic
// ranking - into a single fused ranking. See docs/design/28-rrf.md.
package hybrid

import "sort"

// DefaultK is the damping constant from the original Reciprocal Rank
// Fusion paper (Cormack, Clarke & Buettcher, 2009), also the de facto
// default used by production hybrid-search systems (e.g.
// Elasticsearch). It works well across corpora without per-corpus
// tuning, which is RRF's whole appeal over a weighted score combination.
const DefaultK = 60

// Fuse combines rankings - each a list of stable passage IDs, ordered
// best-first - into one fused ranking via Reciprocal Rank Fusion. Every
// ranking contributes 1/(k + rank) to each ID it contains (rank is
// 1-indexed); an ID's fused score is the sum of that contribution across
// every ranking it appears in. An ID absent from a ranking simply gets
// no contribution from it - RRF only rewards presence and position, it
// never penalizes absence beyond that.
//
// Using rank instead of the underlying scores is the entire point: BM25
// scores and cosine-similarity scores live on incomparable scales, and
// RRF sidesteps that mismatch by never looking at a score, only a
// position.
func Fuse(rankings [][]string, k int) []string {
	scores := make(map[string]float64)
	for _, ranking := range rankings {
		for i, id := range ranking {
			rank := i + 1
			scores[id] += 1.0 / float64(k+rank)
		}
	}

	ids := make([]string, 0, len(scores))
	for id := range scores {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		if scores[ids[i]] != scores[ids[j]] {
			return scores[ids[i]] > scores[ids[j]]
		}
		// Deterministic tiebreak - map iteration order isn't, and a
		// caller re-running the same query should get the same answer.
		return ids[i] < ids[j]
	})
	return ids
}

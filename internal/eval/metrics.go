package eval

// PrecisionAtK is the fraction of the top k ranked documents that are
// relevant (judged relevance >= 1). Ranks beyond the number of results
// actually returned count as non-relevant, so returning fewer than k
// results is never rewarded.
func PrecisionAtK(ranked []int, q Query, k int) float64 {
	relevant := 0
	for i := 0; i < k && i < len(ranked); i++ {
		if q.Relevance(ranked[i]) >= 1 {
			relevant++
		}
	}
	return float64(relevant) / float64(k)
}

// ReciprocalRank is 1/rank of the first relevant document in ranked
// (1-indexed), or 0 if no relevant document appears at all.
func ReciprocalRank(ranked []int, q Query) float64 {
	for i, docID := range ranked {
		if q.Relevance(docID) >= 1 {
			return 1 / float64(i+1)
		}
	}
	return 0
}

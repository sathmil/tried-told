package diskindex

import (
	"sort"

	"triedandtold/internal/tokenize"
)

// PhraseSearch returns the local DocIDs of every passage in this segment
// containing phrase as an exact, consecutive, in-order sequence of words -
// not just passages containing all the words somewhere. Positions are
// what make this possible; BM25 itself never needs them, since it's a
// bag-of-words model that only cares how many times a term occurs, not
// where. See docs/design/22-phrase-search.md.
func (s *Segment) PhraseSearch(phrase string) []int {
	terms := tokenize.Tokenize(phrase)
	if len(terms) == 0 {
		return nil
	}

	// termDocPositions[i] maps docID -> that term's positions in that doc,
	// for term i. Built once so the rest of the search never re-decodes.
	termDocPositions := make([]map[int][]int, len(terms))
	rarestTerm := 0
	for i, term := range terms {
		ps, ok := s.TermPostings(term)
		if !ok {
			return nil // any missing term means no possible phrase match
		}
		m := make(map[int][]int, len(ps))
		for _, p := range ps {
			m[p.LocalDocID] = p.Positions
		}
		termDocPositions[i] = m

		if len(m) < len(termDocPositions[rarestTerm]) {
			rarestTerm = i
		}
	}

	if len(terms) == 1 {
		out := make([]int, 0, len(termDocPositions[0]))
		for docID := range termDocPositions[0] {
			out = append(out, docID)
		}
		sort.Ints(out)
		return out
	}

	// Generate candidates from the rarest term first - a real query
	// optimization, not just a correctness requirement: it minimizes how
	// many documents get checked against the other terms.
	var candidates []int
	for docID := range termDocPositions[rarestTerm] {
		inAll := true
		for i := range termDocPositions {
			if i == rarestTerm {
				continue
			}
			if _, ok := termDocPositions[i][docID]; !ok {
				inAll = false
				break
			}
		}
		if inAll {
			candidates = append(candidates, docID)
		}
	}
	sort.Ints(candidates)

	var matches []int
	for _, docID := range candidates {
		if hasConsecutiveRun(termDocPositions, docID) {
			matches = append(matches, docID)
		}
	}
	return matches
}

// hasConsecutiveRun reports whether, for some starting position p, term i's
// positions in docID include p+i for every i - i.e. the terms occur at
// consecutive, increasing positions, in phrase order.
func hasConsecutiveRun(termDocPositions []map[int][]int, docID int) bool {
	for _, p := range termDocPositions[0][docID] {
		matched := true
		for i := 1; i < len(termDocPositions); i++ {
			if !containsSorted(termDocPositions[i][docID], p+i) {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}

// containsSorted reports whether v is present in sorted, using binary
// search - positions within a posting are always sorted, a property
// maintained since design doc 01.
func containsSorted(sorted []int, v int) bool {
	i := sort.SearchInts(sorted, v)
	return i < len(sorted) && sorted[i] == v
}

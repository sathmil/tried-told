package diskindex

import "sort"

// Deleter reports whether a passage (identified by its stable
// extract.Passage.ID()) has been deleted. *crawlstate.DeletionLog
// satisfies this directly. Defined locally, matching the identically-shaped
// bm25.Deleter and semantic.Deleter, so this package doesn't import
// either just to name a one-method interface.
type Deleter interface {
	IsDeleted(passageID string) bool
}

// MergeSegments combines segs into one new segment file at path,
// permanently dropping any passage deleted (as reported by deleted) -
// its postings, doc-length entry, and ID-mapping entry are all simply
// left out, not merely hidden. This is what actually reclaims space for
// a tombstoned passage: crawlstate.DeletionLog only filters it out of
// query results until the segments holding it are merged. See
// docs/design/31-segment-merging.md.
//
// Segments carry no passage text (only postings, doc lengths, and an
// ID mapping), so merging can't re-tokenize the way BuildSegment does -
// it has to decode and recombine each segment's existing postings
// instead, which is why this lives here rather than as a thin wrapper
// around BuildSegment.
func MergeSegments(segs []*Segment, deleted Deleter, path string) error {
	// Assign each surviving passage a fresh, contiguous global ID, in
	// segment order. Recording each segment's old-local-ID -> new-global-ID
	// mapping (-1 for a dropped, tombstoned passage) up front means every
	// later step - remapping a term's postings - is a single lookup, not
	// a repeated search.
	remaps := make([][]int, len(segs))
	var passageIDs []string
	var docLens []int

	nextID := 0
	for i, seg := range segs {
		remap := make([]int, seg.N())
		for old := 0; old < seg.N(); old++ {
			pid := seg.PassageID(old)
			if deleted.IsDeleted(pid) {
				remap[old] = -1
				continue
			}
			remap[old] = nextID
			nextID++
			passageIDs = append(passageIDs, pid)
			docLens = append(docLens, seg.DocLen(old))
		}
		remaps[i] = remap
	}

	termSet := make(map[string]struct{})
	for _, seg := range segs {
		for _, t := range seg.Terms() {
			termSet[t] = struct{}{}
		}
	}

	termDocPositions := make(map[string]map[int][]int, len(termSet))
	for term := range termSet {
		docMap := make(map[int][]int)
		for i, seg := range segs {
			postingsList, ok := seg.TermPostings(term)
			if !ok {
				continue
			}
			for _, p := range postingsList {
				newID := remaps[i][p.LocalDocID]
				if newID == -1 {
					continue // this passage was tombstoned - drop it, not just hide it
				}
				docMap[newID] = p.Positions
			}
		}
		// A term whose every occurrence was in tombstoned passages has
		// nothing left to record - dropping it entirely (rather than
		// keeping a df=0 entry) is part of actually reclaiming space,
		// not just reclaiming it for passages.
		if len(docMap) > 0 {
			termDocPositions[term] = docMap
		}
	}

	terms := make([]string, 0, len(termDocPositions))
	for t := range termDocPositions {
		terms = append(terms, t)
	}
	sort.Strings(terms)

	return writeSegment(terms, termDocPositions, docLens, passageIDs, path)
}

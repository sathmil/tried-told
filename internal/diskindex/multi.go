package diskindex

import "sort"

// MultiSegment combines several immutable Segments into one queryable
// unit, presenting a single unified DocID space across all of them - the
// piece that makes incremental indexing actually useful: writing a new
// segment for newly-arrived passages is pointless if nothing queries it
// alongside the existing ones. See docs/design/24-multi-segment.md.
type MultiSegment struct {
	segments  []*Segment
	base      []int // base[i] = the first global DocID belonging to segments[i]
	n         int
	avgDocLen float64
}

// NewMultiSegment combines already-open segments, in the given order.
func NewMultiSegment(segments []*Segment) *MultiSegment {
	base := make([]int, len(segments))
	total := 0
	weightedLenSum := 0.0
	for i, s := range segments {
		base[i] = total
		total += s.N()
		weightedLenSum += s.AvgDocLen() * float64(s.N())
	}
	avg := 0.0
	if total > 0 {
		avg = weightedLenSum / float64(total)
	}
	return &MultiSegment{segments: segments, base: base, n: total, avgDocLen: avg}
}

// OpenMultiSegment opens every segment file at paths, in order, and
// combines them.
func OpenMultiSegment(paths []string) (*MultiSegment, error) {
	segments := make([]*Segment, len(paths))
	for i, path := range paths {
		seg, err := OpenSegment(path)
		if err != nil {
			return nil, err
		}
		segments[i] = seg
	}
	return NewMultiSegment(segments), nil
}

// N returns the total number of passages across every constituent segment.
func (m *MultiSegment) N() int { return m.n }

// AvgDocLen returns the length-weighted average document length across
// every constituent segment, for BM25.
func (m *MultiSegment) AvgDocLen() float64 { return m.avgDocLen }

// locate translates a global DocID into which segment owns it and that
// segment's own local ID for it.
func (m *MultiSegment) locate(globalID int) (segIdx, localID int) {
	// base is ascending by construction; the owning segment is the last
	// one whose base is <= globalID.
	i := sort.SearchInts(m.base, globalID+1) - 1
	return i, globalID - m.base[i]
}

// DocLen returns the token count of the passage at globalID.
func (m *MultiSegment) DocLen(globalID int) int {
	segIdx, localID := m.locate(globalID)
	return m.segments[segIdx].DocLen(localID)
}

// PassageID returns the real extract.Passage.ID() for globalID.
func (m *MultiSegment) PassageID(globalID int) string {
	segIdx, localID := m.locate(globalID)
	return m.segments[segIdx].PassageID(localID)
}

// TermPostings decodes and combines term's postings across every
// constituent segment, translating each into the unified global ID space.
func (m *MultiSegment) TermPostings(term string) ([]Posting, bool) {
	var out []Posting
	for i, s := range m.segments {
		ps, ok := s.TermPostings(term)
		if !ok {
			continue
		}
		for _, p := range ps {
			out = append(out, Posting{LocalDocID: m.base[i] + p.LocalDocID, Positions: p.Positions})
		}
	}
	if len(out) == 0 {
		return nil, false
	}
	return out, true
}

// PhraseSearch returns the global DocIDs of every passage, across every
// constituent segment, containing phrase as an exact, consecutive,
// in-order sequence of words - same semantics as Segment.PhraseSearch,
// combined across segments.
func (m *MultiSegment) PhraseSearch(phrase string) []int {
	var out []int
	for i, s := range m.segments {
		for _, localID := range s.PhraseSearch(phrase) {
			out = append(out, m.base[i]+localID)
		}
	}
	sort.Ints(out)
	return out
}

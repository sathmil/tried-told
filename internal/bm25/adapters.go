package bm25

import (
	"triedandtold/internal/diskindex"
	"triedandtold/internal/index"
)

// WrapInMemory adapts index.Index (Milestone 1's in-memory index) to the
// Index interface Search needs.
func WrapInMemory(idx index.Index) Index {
	return inMemoryIndex{idx: idx}
}

type inMemoryIndex struct {
	idx index.Index
}

func (w inMemoryIndex) Postings(term string) ([]Posting, bool) {
	ps, ok := w.idx.Postings[term]
	if !ok {
		return nil, false
	}
	out := make([]Posting, len(ps))
	for i, p := range ps {
		out[i] = Posting{DocID: p.DocID, Freq: p.Freq}
	}
	return out, true
}

func (w inMemoryIndex) N() int               { return w.idx.N }
func (w inMemoryIndex) AvgDocLen() float64   { return w.idx.AvgDocLen }
func (w inMemoryIndex) DocLen(docID int) int { return w.idx.DocLen[docID] }

// WrapSegment adapts a disk-backed diskindex.Queryable - a single Segment
// or a MultiSegment combining several - to the Index interface Search
// needs. Both satisfy Queryable with the same method set, so this one
// adapter handles either without needing a separate wrapper for each.
// A segment's postings store word positions, not a frequency field
// directly - Freq here is derived as len(Positions), same as everywhere
// else positions are used instead of a separate count.
func WrapSegment(seg diskindex.Queryable) Index {
	return segmentIndex{seg: seg}
}

type segmentIndex struct {
	seg diskindex.Queryable
}

func (w segmentIndex) Postings(term string) ([]Posting, bool) {
	ps, ok := w.seg.TermPostings(term)
	if !ok {
		return nil, false
	}
	out := make([]Posting, len(ps))
	for i, p := range ps {
		out[i] = Posting{DocID: p.LocalDocID, Freq: len(p.Positions)}
	}
	return out, true
}

func (w segmentIndex) N() int               { return w.seg.N() }
func (w segmentIndex) AvgDocLen() float64   { return w.seg.AvgDocLen() }
func (w segmentIndex) DocLen(docID int) int { return w.seg.DocLen(docID) }

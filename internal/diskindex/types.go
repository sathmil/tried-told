// Package diskindex implements one immutable, disk-backed index segment:
// a term dictionary, delta+varint-compressed postings with word
// positions, per-document lengths, and a local-ID-to-Passage-ID mapping.
// Per docs/design/20-segment-format.md.
package diskindex

// magic identifies this file format and its version.
var magic = [4]byte{'T', 'T', 'S', '1'}

// header is the segment file's fixed-size preamble. encoding/binary
// serializes struct fields in declared order with no padding (verified via
// go doc, not assumed), so this can be written/read directly with
// binary.Write/Read.
type header struct {
	Magic       [4]byte
	N           uint32  // number of passages in this segment
	AvgDocLen   float64 // for BM25 length normalization
	PostingsOff uint64
	PostingsLen uint64
	DictOff     uint64
	DictLen     uint64
	DocLensOff  uint64
	DocLensLen  uint64
	IDMapOff    uint64
	IDMapLen    uint64
}

// dictEntry is one term dictionary record: where this term's postings live
// within the postings section (offset relative to the section start, not
// the file), how many bytes they occupy, and its document frequency
// (stored explicitly so IDF doesn't require decoding postings just to
// count them).
type dictEntry struct {
	offset uint64
	length uint64
	df     int
}

// passageIDSize is the fixed byte width of one ID-mapping entry:
// extract.Passage.ID() is always a 64-character hex SHA-256 digest.
const passageIDSize = 64

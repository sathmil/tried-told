package diskindex

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"os"

	"triedandtold/internal/postings"
)

// Posting is one decoded posting: the local DocID and the token positions
// where the term occurred in that document. Freq is len(Positions) - it
// has no separate field, since the position count already implies it.
type Posting struct {
	LocalDocID int
	Positions  []int
}

// Segment is an opened, read-only index segment. The whole file is loaded
// into memory at open time rather than read on demand via seeks - at this
// project's corpus scale (~16MB raw for 100,000 passages, per
// docs/design/20-segment-format.md) that's simpler and still well within
// budget. Revisit only if corpus scale grows enough to make that untrue.
type Segment struct {
	n            int
	avgDocLen    float64
	dict         map[string]dictEntry
	docLens      []int
	passageIDs   []string
	postingsData []byte
}

// OpenSegment opens the segment file at path, verifying its checksum
// before trusting any of its contents, and loads the dictionary,
// doc-lengths, and ID-mapping sections into memory.
func OpenSegment(path string) (*Segment, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) < 4 {
		return nil, fmt.Errorf("diskindex: %s is too small to be a valid segment", path)
	}

	body, checksumBytes := data[:len(data)-4], data[len(data)-4:]
	want := binary.LittleEndian.Uint32(checksumBytes)
	got := crc32.ChecksumIEEE(body)
	if got != want {
		return nil, fmt.Errorf("diskindex: %s failed checksum verification (corrupt file): got %08x, want %08x", path, got, want)
	}

	var h header
	if err := binary.Read(bytes.NewReader(body), binary.LittleEndian, &h); err != nil {
		return nil, fmt.Errorf("diskindex: reading header: %w", err)
	}
	if h.Magic != magic {
		return nil, fmt.Errorf("diskindex: %s has invalid magic bytes, not a segment file", path)
	}

	dict, err := decodeDictionary(body[h.DictOff : h.DictOff+h.DictLen])
	if err != nil {
		return nil, fmt.Errorf("diskindex: decoding dictionary: %w", err)
	}

	return &Segment{
		n:            int(h.N),
		avgDocLen:    h.AvgDocLen,
		dict:         dict,
		docLens:      decodeDocLens(body[h.DocLensOff : h.DocLensOff+h.DocLensLen]),
		passageIDs:   decodeIDMap(body[h.IDMapOff : h.IDMapOff+h.IDMapLen]),
		postingsData: body[h.PostingsOff : h.PostingsOff+h.PostingsLen],
	}, nil
}

// N returns the number of passages in this segment.
func (s *Segment) N() int { return s.n }

// AvgDocLen returns the segment's average document length, for BM25.
func (s *Segment) AvgDocLen() float64 { return s.avgDocLen }

// DocLen returns the token count of the passage at localID.
func (s *Segment) DocLen(localID int) int { return s.docLens[localID] }

// PassageID returns the real extract.Passage.ID() for localID - the join
// key back to deletion tombstones and attribution lookups.
func (s *Segment) PassageID(localID int) string { return s.passageIDs[localID] }

// Terms returns every term in this segment's dictionary, in no
// particular order. Used by MergeSegments to build the union of terms
// across multiple segments before recombining their postings.
func (s *Segment) Terms() []string {
	terms := make([]string, 0, len(s.dict))
	for t := range s.dict {
		terms = append(terms, t)
	}
	return terms
}

// DocFreq returns the number of documents term appears in within this
// segment, or 0 if it doesn't appear at all.
func (s *Segment) DocFreq(term string) int {
	e, ok := s.dict[term]
	if !ok {
		return 0
	}
	return e.df
}

// TermPostings decodes and returns term's full postings list, or
// (nil, false) if the term doesn't appear in this segment.
//
// A decode failure here panics rather than returning an error: the
// checksum already verified these exact bytes at OpenSegment time, so a
// failure decoding them indicates a bug in the encode/decode logic itself,
// not a normal, recoverable runtime condition - the same "should never
// happen if the code is correct" reasoning as BuildIndex's ID-contiguity
// check.
func (s *Segment) TermPostings(term string) ([]Posting, bool) {
	e, ok := s.dict[term]
	if !ok {
		return nil, false
	}
	block := s.postingsData[e.offset : e.offset+e.length]

	docCount, n := binary.Uvarint(block)
	if n <= 0 {
		panic(fmt.Sprintf("diskindex: corrupt postings block for term %q despite a valid checksum", term))
	}
	block = block[n:]

	docIDs, consumed, err := postings.DecodeN(block, int(docCount))
	if err != nil {
		panic(fmt.Sprintf("diskindex: corrupt postings block for term %q despite a valid checksum: %v", term, err))
	}
	block = block[consumed:]

	result := make([]Posting, len(docIDs))
	for i, docID := range docIDs {
		posCount, n := binary.Uvarint(block)
		if n <= 0 {
			panic(fmt.Sprintf("diskindex: corrupt postings block for term %q despite a valid checksum", term))
		}
		block = block[n:]

		positions, consumed, err := postings.DecodeN(block, int(posCount))
		if err != nil {
			panic(fmt.Sprintf("diskindex: corrupt postings block for term %q despite a valid checksum: %v", term, err))
		}
		block = block[consumed:]

		result[i] = Posting{LocalDocID: docID, Positions: positions}
	}

	return result, true
}

func decodeDictionary(data []byte) (map[string]dictEntry, error) {
	const fixedFieldsSize = 8 + 8 // offset (uint64) + length (uint64)

	count, n := binary.Uvarint(data)
	if n <= 0 {
		return nil, fmt.Errorf("corrupt term count")
	}
	data = data[n:]

	dict := make(map[string]dictEntry, count)
	for i := uint64(0); i < count; i++ {
		termLen, n := binary.Uvarint(data)
		if n <= 0 {
			return nil, fmt.Errorf("entry %d: corrupt term length", i)
		}
		data = data[n:]

		if uint64(len(data)) < termLen {
			return nil, fmt.Errorf("entry %d: term truncated", i)
		}
		term := string(data[:termLen])
		data = data[termLen:]

		if len(data) < fixedFieldsSize {
			return nil, fmt.Errorf("entry %d: truncated offset/length", i)
		}
		offset := binary.LittleEndian.Uint64(data[0:8])
		length := binary.LittleEndian.Uint64(data[8:16])
		data = data[fixedFieldsSize:]

		df, n := binary.Uvarint(data)
		if n <= 0 {
			return nil, fmt.Errorf("entry %d: corrupt document frequency", i)
		}
		data = data[n:]

		dict[term] = dictEntry{offset: offset, length: length, df: int(df)}
	}
	return dict, nil
}

func decodeDocLens(data []byte) []int {
	n := len(data) / 4
	out := make([]int, n)
	for i := 0; i < n; i++ {
		out[i] = int(binary.LittleEndian.Uint32(data[i*4 : i*4+4]))
	}
	return out
}

func decodeIDMap(data []byte) []string {
	n := len(data) / passageIDSize
	out := make([]string, n)
	for i := 0; i < n; i++ {
		out[i] = string(data[i*passageIDSize : (i+1)*passageIDSize])
	}
	return out
}

package diskindex

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"sort"

	"triedandtold/internal/extract"
	"triedandtold/internal/postings"
	"triedandtold/internal/tokenize"
)

// BuildSegment tokenizes each passage, builds delta+varint-compressed
// postings with word positions, and writes one immutable segment file to
// path.
//
// Passages are assigned sequential local IDs (0..N-1) so postings can be
// delta-encoded - extract.Passage.ID() is a 64-character hash with no
// numeric structure, and using it directly as a posting's DocID would
// destroy the compression internal/postings provides. Each local ID's
// real Passage.ID() is stored in the ID-mapping section, so deletion
// tombstones and attribution lookups still work against segment contents.
func BuildSegment(passages []extract.Passage, path string) error {
	n := len(passages)

	docLens := make([]int, n)
	passageIDs := make([]string, n)
	termDocPositions := make(map[string]map[int][]int)
	totalLen := 0

	for localID, p := range passages {
		passageIDs[localID] = p.ID()
		tokens := tokenize.Tokenize(p.Text)
		docLens[localID] = len(tokens)
		totalLen += len(tokens)

		for pos, tok := range tokens {
			docs := termDocPositions[tok]
			if docs == nil {
				docs = make(map[int][]int)
				termDocPositions[tok] = docs
			}
			docs[localID] = append(docs[localID], pos)
		}
	}

	avgDocLen := 0.0
	if n > 0 {
		avgDocLen = float64(totalLen) / float64(n)
	}

	terms := make([]string, 0, len(termDocPositions))
	for t := range termDocPositions {
		terms = append(terms, t)
	}
	sort.Strings(terms)

	postingsBuf, dict := encodePostings(terms, termDocPositions)
	dictBuf := encodeDictionary(terms, dict)
	docLensBuf := encodeDocLens(docLens)
	idMapBuf, err := encodeIDMap(passageIDs)
	if err != nil {
		return err
	}

	h := header{
		Magic:       magic,
		N:           uint32(n),
		AvgDocLen:   avgDocLen,
		PostingsOff: uint64(headerSize()),
		PostingsLen: uint64(len(postingsBuf)),
	}
	h.DictOff = h.PostingsOff + h.PostingsLen
	h.DictLen = uint64(len(dictBuf))
	h.DocLensOff = h.DictOff + h.DictLen
	h.DocLensLen = uint64(len(docLensBuf))
	h.IDMapOff = h.DocLensOff + h.DocLensLen
	h.IDMapLen = uint64(len(idMapBuf))

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	hasher := crc32.NewIEEE()
	mw := io.MultiWriter(w, hasher) // every byte written also feeds the checksum

	if err := binary.Write(mw, binary.LittleEndian, h); err != nil {
		return fmt.Errorf("diskindex: writing header: %w", err)
	}
	for _, section := range [][]byte{postingsBuf, dictBuf, docLensBuf, idMapBuf} {
		if _, err := mw.Write(section); err != nil {
			return fmt.Errorf("diskindex: writing segment data: %w", err)
		}
	}

	// The checksum covers everything written above it, so it must itself
	// be written directly to w, not through mw (which would make it
	// checksum itself).
	if err := binary.Write(w, binary.LittleEndian, hasher.Sum32()); err != nil {
		return fmt.Errorf("diskindex: writing checksum: %w", err)
	}

	return w.Flush()
}

func headerSize() int {
	return binary.Size(header{})
}

// encodePostings writes, for each term in order: a varint doc count, the
// delta+varint-encoded DocID gaps, then per DocID a varint position count
// and the delta+varint-encoded position gaps. Returns the encoded bytes
// and each term's (offset, length, df) within them.
func encodePostings(terms []string, termDocPositions map[string]map[int][]int) ([]byte, map[string]dictEntry) {
	var buf []byte
	dict := make(map[string]dictEntry, len(terms))

	for _, term := range terms {
		docMap := termDocPositions[term]
		docIDs := make([]int, 0, len(docMap))
		for id := range docMap {
			docIDs = append(docIDs, id)
		}
		sort.Ints(docIDs)

		start := len(buf)

		buf = binary.AppendUvarint(buf, uint64(len(docIDs)))
		buf = append(buf, postings.EncodeDeltas(docIDs)...)

		for _, id := range docIDs {
			pos := docMap[id] // already increasing: tokens are appended in scan order
			buf = binary.AppendUvarint(buf, uint64(len(pos)))
			buf = append(buf, postings.EncodeDeltas(pos)...)
		}

		dict[term] = dictEntry{
			offset: uint64(start),
			length: uint64(len(buf) - start),
			df:     len(docIDs),
		}
	}

	return buf, dict
}

// encodeDictionary writes: a varint term count, then per term (sorted, for
// determinism) a varint length-prefixed term string followed by its
// dictEntry (offset/length relative to the postings section, plus df).
func encodeDictionary(terms []string, dict map[string]dictEntry) []byte {
	var buf []byte
	buf = binary.AppendUvarint(buf, uint64(len(terms)))
	for _, term := range terms {
		e := dict[term]
		buf = binary.AppendUvarint(buf, uint64(len(term)))
		buf = append(buf, term...)
		buf = binary.LittleEndian.AppendUint64(buf, e.offset)
		buf = binary.LittleEndian.AppendUint64(buf, e.length)
		buf = binary.AppendUvarint(buf, uint64(e.df))
	}
	return buf
}

// encodeDocLens writes one fixed-width uint32 per local DocID, in order.
// Fixed-width (not varint) deliberately: doc lengths need random access by
// local ID (BM25 needs the length of whichever doc a posting references),
// so entry i must live at a computable offset i*4 - varint's whole point
// is variable width, which is exactly incompatible with that.
func encodeDocLens(docLens []int) []byte {
	buf := make([]byte, 0, len(docLens)*4)
	for _, l := range docLens {
		buf = binary.LittleEndian.AppendUint32(buf, uint32(l))
	}
	return buf
}

// encodeIDMap writes one fixed-width passageIDSize-byte record per local
// DocID, in order - same random-access reasoning as encodeDocLens.
// extract.Passage.ID() is always exactly passageIDSize characters (a hex
// SHA-256 digest), so this is naturally fixed-width already.
func encodeIDMap(passageIDs []string) ([]byte, error) {
	buf := make([]byte, 0, len(passageIDs)*passageIDSize)
	for _, id := range passageIDs {
		if len(id) != passageIDSize {
			return nil, fmt.Errorf("diskindex: passage ID %q is %d bytes, want exactly %d", id, len(id), passageIDSize)
		}
		buf = append(buf, id...)
	}
	return buf, nil
}

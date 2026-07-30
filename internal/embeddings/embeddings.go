// Package embeddings reads the binary embeddings file format (TTE1)
// produced offline by python/generate_embeddings.py. See
// docs/design/25-embeddings-format.md.
package embeddings

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"math"
	"os"
	"time"
)

var magic = [4]byte{'T', 'T', 'E', '1'}

const passageIDSize = 64

// header is the file's fixed-size preamble. encoding/binary serializes
// struct fields in declared order with no padding (verified via go doc,
// same as internal/diskindex) - and Python's struct.pack with "<" (little-
// endian, standard sizes) produces byte-for-byte identical output, which
// is what actually makes this a real, working cross-language format
// rather than an assumption. See docs/design/25 for the round-trip proof.
type header struct {
	Magic           [4]byte
	Dim             uint32
	Count           uint32
	GeneratedAtUnix int64
	VersionOff      uint64
	VersionLen      uint64
	RecordsOff      uint64
	RecordsLen      uint64
}

// Embedding is one passage's embedding vector.
type Embedding struct {
	PassageID string
	Vector    []float32
}

// File is a fully-loaded embeddings file.
type File struct {
	Version     string
	Dim         int
	GeneratedAt time.Time
	Embeddings  []Embedding
}

// Open reads and verifies the embeddings file at path, checking its
// checksum before trusting any of its contents - same discipline as
// diskindex.OpenSegment.
func Open(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) < 4 {
		return nil, fmt.Errorf("embeddings: %s is too small to be a valid embeddings file", path)
	}

	body, checksumBytes := data[:len(data)-4], data[len(data)-4:]
	want := binary.LittleEndian.Uint32(checksumBytes)
	got := crc32.ChecksumIEEE(body)
	if got != want {
		return nil, fmt.Errorf("embeddings: %s failed checksum verification (corrupt file): got %08x, want %08x", path, got, want)
	}

	var h header
	if err := binary.Read(bytes.NewReader(body), binary.LittleEndian, &h); err != nil {
		return nil, fmt.Errorf("embeddings: reading header: %w", err)
	}
	if h.Magic != magic {
		return nil, fmt.Errorf("embeddings: %s has invalid magic bytes, not an embeddings file", path)
	}

	version := string(body[h.VersionOff : h.VersionOff+h.VersionLen])

	dim := int(h.Dim)
	recordSize := passageIDSize + dim*4
	recordsData := body[h.RecordsOff : h.RecordsOff+h.RecordsLen]

	count := int(h.Count)
	list := make([]Embedding, count)
	for i := 0; i < count; i++ {
		rec := recordsData[i*recordSize : (i+1)*recordSize]
		passageID := string(rec[:passageIDSize])

		vecBytes := rec[passageIDSize:]
		vec := make([]float32, dim)
		for j := 0; j < dim; j++ {
			bits := binary.LittleEndian.Uint32(vecBytes[j*4 : j*4+4])
			vec[j] = math.Float32frombits(bits)
		}

		list[i] = Embedding{PassageID: passageID, Vector: vec}
	}

	return &File{
		Version:     version,
		Dim:         dim,
		GeneratedAt: time.Unix(h.GeneratedAtUnix, 0).UTC(),
		Embeddings:  list,
	}, nil
}

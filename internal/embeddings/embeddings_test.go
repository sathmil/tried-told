package embeddings

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTestFile hand-constructs a valid TTE1 file, mirroring exactly what
// python/generate_embeddings.py writes, so Open can be tested without
// requiring Python/sentence-transformers to be installed.
func writeTestFile(t *testing.T, path, version string, ids []string, vectors [][]float32) {
	t.Helper()

	dim := len(vectors[0])
	versionBytes := []byte(version)

	var buf bytes.Buffer
	h := header{
		Magic:           magic,
		Dim:             uint32(dim),
		Count:           uint32(len(ids)),
		GeneratedAtUnix: 1234567890,
		VersionOff:      uint64(binary.Size(header{})),
		VersionLen:      uint64(len(versionBytes)),
	}
	h.RecordsOff = h.VersionOff + h.VersionLen
	h.RecordsLen = uint64(len(ids) * (passageIDSize + dim*4))

	if err := binary.Write(&buf, binary.LittleEndian, h); err != nil {
		t.Fatalf("writing header: %v", err)
	}
	buf.Write(versionBytes)
	for i, id := range ids {
		if len(id) != passageIDSize {
			t.Fatalf("test id %q is not %d chars", id, passageIDSize)
		}
		buf.WriteString(id)
		for _, v := range vectors[i] {
			var b [4]byte
			binary.LittleEndian.PutUint32(b[:], math.Float32bits(v))
			buf.Write(b[:])
		}
	}

	checksum := crc32.ChecksumIEEE(buf.Bytes())
	var checksumBytes [4]byte
	binary.LittleEndian.PutUint32(checksumBytes[:], checksum)
	buf.Write(checksumBytes[:])

	if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
		t.Fatalf("writing test file: %v", err)
	}
}

func TestOpen_RoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.bin")
	ids := []string{
		strings.Repeat("1", passageIDSize),
		strings.Repeat("2", passageIDSize),
	}
	vectors := [][]float32{
		{0.1, 0.2, 0.3},
		{-0.4, 0.5, -0.6},
	}
	writeTestFile(t, path, "test-version-1", ids, vectors)

	f, err := Open(path)
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}

	if f.Version != "test-version-1" {
		t.Errorf("Version = %q, want %q", f.Version, "test-version-1")
	}
	if f.Dim != 3 {
		t.Errorf("Dim = %d, want 3", f.Dim)
	}
	if len(f.Embeddings) != 2 {
		t.Fatalf("got %d embeddings, want 2", len(f.Embeddings))
	}
	for i, want := range vectors {
		if f.Embeddings[i].PassageID != ids[i] {
			t.Errorf("embedding %d PassageID = %q, want %q", i, f.Embeddings[i].PassageID, ids[i])
		}
		for j, v := range want {
			if f.Embeddings[i].Vector[j] != v {
				t.Errorf("embedding %d vector[%d] = %v, want %v", i, j, f.Embeddings[i].Vector[j], v)
			}
		}
	}
}

func TestOpen_CorruptionIsDetected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.bin")
	ids := []string{strings.Repeat("3", passageIDSize)}
	vectors := [][]float32{{0.1, 0.2}}
	writeTestFile(t, path, "v1", ids, vectors)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading test file: %v", err)
	}
	data[len(data)/2] ^= 0xFF
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("writing corrupted file: %v", err)
	}

	if _, err := Open(path); err == nil {
		t.Error("Open on a corrupted file returned no error")
	}
}

// TestOpen_RealPythonGeneratedFile proves the reader works against
// genuine output from python/generate_embeddings.py, not just Go-authored
// bytes - the fixture was generated for real, once, and committed rather
// than regenerated at test time, since that would require Python and the
// BGE model to be available wherever tests run.
func TestOpen_RealPythonGeneratedFile(t *testing.T) {
	f, err := Open("testdata/sample.bin")
	if err != nil {
		t.Fatalf("Open returned error: %v", err)
	}

	if f.Version != "bge-small-en-v1.5-cosine-normalized-v1" {
		t.Errorf("Version = %q, want the real generation script's version string", f.Version)
	}
	if f.Dim != 384 {
		t.Errorf("Dim = %d, want 384 (bge-small-en-v1.5's real dimension)", f.Dim)
	}
	if len(f.Embeddings) != 3 {
		t.Fatalf("got %d embeddings, want 3", len(f.Embeddings))
	}

	for _, e := range f.Embeddings {
		if len(e.PassageID) != 64 {
			t.Errorf("PassageID %q is %d chars, want 64", e.PassageID, len(e.PassageID))
		}
		if len(e.Vector) != 384 {
			t.Fatalf("vector for %s has %d dims, want 384", e.PassageID, len(e.Vector))
		}

		var normSq float64
		for _, v := range e.Vector {
			normSq += float64(v) * float64(v)
		}
		norm := math.Sqrt(normSq)
		if math.Abs(norm-1.0) > 1e-4 {
			t.Errorf("vector for %s has L2 norm %v, want ~1.0 (normalize_embeddings=True)", e.PassageID, norm)
		}
	}
}

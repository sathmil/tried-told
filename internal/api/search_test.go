package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"triedandtold/internal/bm25"
	"triedandtold/internal/diskindex"
	"triedandtold/internal/embeddings"
	"triedandtold/internal/extract"
	"triedandtold/internal/index"
	"triedandtold/internal/semantic"
)

// fakeEmbedder lets tests exercise the hybrid path without a running
// embed_service.py.
type fakeEmbedder struct {
	vector []float32
	err    error
}

func (f fakeEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	return f.vector, f.err
}

// setup builds a real segment-backed SearchDeps (no semantic search) from
// two passages - segment-backed because that's what SearchHandler now
// requires (it needs Resolver.PassageID, which only the disk-backed
// format provides).
func setup(t *testing.T) SearchDeps {
	t.Helper()

	docs := []index.IndexDoc{
		{ID: 0, Text: "sunscreen leaves white cast"},
		{ID: 1, Text: "moisturizer is great"},
	}
	metas := []index.DocMeta{
		{ID: 0, Source: "synthetic", Product: "sunscreen", SkinTone: "deep"},
		{ID: 1, Source: "synthetic", Product: "moisturizer"},
	}
	passages := []extract.Passage{
		{Text: docs[0].Text, SourceURL: metas[0].Source, Product: metas[0].Product, SkinTone: metas[0].SkinTone},
		{Text: docs[1].Text, SourceURL: metas[1].Source, Product: metas[1].Product},
	}

	segPath := filepath.Join(t.TempDir(), "test.seg")
	if err := diskindex.BuildSegment(passages, segPath); err != nil {
		t.Fatalf("BuildSegment returned error: %v", err)
	}
	segment, err := diskindex.OpenSegment(segPath)
	if err != nil {
		t.Fatalf("OpenSegment returned error: %v", err)
	}

	reverseID := make(map[string]int, len(passages))
	for i, p := range passages {
		reverseID[p.ID()] = i
	}

	return SearchDeps{
		Lexical:   bm25.WrapSegment(segment),
		Resolver:  segment,
		ReverseID: reverseID,
		Docs:      docs,
		Metas:     metas,
		Params:    bm25.DefaultParams,
	}
}

func TestSearchHandler_MissingQueryReturns400(t *testing.T) {
	handler := SearchHandler(setup(t))

	req := httptest.NewRequest(http.MethodGet, "/search", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestSearchHandler_ReturnsMatchingResults(t *testing.T) {
	handler := SearchHandler(setup(t))

	req := httptest.NewRequest(http.MethodGet, "/search?q=sunscreen", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var results []searchResult
	if err := json.Unmarshal(rec.Body.Bytes(), &results); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(results) != 1 || results[0].DocID != 0 {
		t.Errorf("results = %v, want exactly doc 0", results)
	}
	if results[0].Product != "sunscreen" || results[0].SkinTone != "deep" {
		t.Errorf("results[0] = %+v, want product=sunscreen skin_tone=deep", results[0])
	}
}

func TestSearchHandler_NoMatchesReturnsEmptyArray(t *testing.T) {
	handler := SearchHandler(setup(t))

	req := httptest.NewRequest(http.MethodGet, "/search?q=nonexistentterm", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var results []searchResult
	if err := json.Unmarshal(rec.Body.Bytes(), &results); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("results = %v, want empty array", results)
	}
}

// TestSearchHandler_FusesLexicalAndSemanticResults proves the actual
// point of this wiring: a query that only matches doc 1 lexically, but
// whose (faked) embedding is nearest to doc 0's, should surface both -
// not just whichever ranking runs first.
func TestSearchHandler_FusesLexicalAndSemanticResults(t *testing.T) {
	deps := setup(t)

	embFile := &embeddings.File{Embeddings: []embeddings.Embedding{
		{PassageID: passageIDForDoc(t, deps, 0), Vector: []float32{1, 0}},
		{PassageID: passageIDForDoc(t, deps, 1), Vector: []float32{0, 1}},
	}}
	semIdx, err := semantic.Open(filepath.Join(t.TempDir(), "test.graph"))
	if err != nil {
		t.Fatalf("semantic.Open returned error: %v", err)
	}
	if err := semIdx.Add(embFile); err != nil {
		t.Fatalf("semIdx.Add returned error: %v", err)
	}
	deps.Semantic = semIdx
	deps.Embedder = fakeEmbedder{vector: []float32{1, 0}} // nearest to doc 0

	handler := SearchHandler(deps)
	req := httptest.NewRequest(http.MethodGet, "/search?q=moisturizer", nil) // matches doc 1 lexically only
	rec := httptest.NewRecorder()
	handler(rec, req)

	var results []searchResult
	if err := json.Unmarshal(rec.Body.Bytes(), &results); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("results = %v, want both docs (one lexical-only, one semantic-only)", results)
	}
}

// TestSearchHandler_FallsBackToBM25OnlyWhenEmbedderFails proves semantic
// search is an enrichment, not a hard dependency: an Embed error must not
// turn into a failed request.
func TestSearchHandler_FallsBackToBM25OnlyWhenEmbedderFails(t *testing.T) {
	deps := setup(t)
	semIdx, err := semantic.Open(filepath.Join(t.TempDir(), "test.graph"))
	if err != nil {
		t.Fatalf("semantic.Open returned error: %v", err)
	}
	deps.Semantic = semIdx
	deps.Embedder = fakeEmbedder{err: errors.New("embed service unreachable")}

	handler := SearchHandler(deps)
	req := httptest.NewRequest(http.MethodGet, "/search?q=sunscreen", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (embed failure should degrade, not fail, the request)", rec.Code, http.StatusOK)
	}
	var results []searchResult
	if err := json.Unmarshal(rec.Body.Bytes(), &results); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(results) != 1 || results[0].DocID != 0 {
		t.Errorf("results = %v, want the BM25-only match (doc 0)", results)
	}
}

// passageIDForDoc looks up which Passage.ID() setup() assigned to a given
// DocID, so tests can build fixtures keyed the same way the handler
// expects.
func passageIDForDoc(t *testing.T, deps SearchDeps, docID int) string {
	t.Helper()
	for pid, id := range deps.ReverseID {
		if id == docID {
			return pid
		}
	}
	t.Fatalf("no passage ID found for docID %d", docID)
	return ""
}

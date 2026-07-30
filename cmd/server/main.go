// Command server runs the Tried & Told search HTTP service.
package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"triedandtold/internal/api"
	"triedandtold/internal/bm25"
	"triedandtold/internal/corpus"
	"triedandtold/internal/diskindex"
	"triedandtold/internal/embedclient"
	"triedandtold/internal/embeddings"
	"triedandtold/internal/semantic"
)

//go:embed static
var staticFS embed.FS

const (
	corpusPath      = "data/synthetic/experiences.jsonl"
	embeddingsPath  = "data/embeddings/synthetic.bin"
	embedServiceURL = "http://127.0.0.1:8091"
)

func main() {
	docs, metas, err := corpus.LoadJSONL(corpusPath)
	if err != nil {
		log.Fatalf("loading corpus: %v", err)
	}
	passages := corpus.ToPassages(docs, metas)

	// The lexical segment and HNSW graph are rebuilt fresh into a temp
	// dir on every startup - the synthetic corpus is small enough that
	// there's no reason to persist either across restarts.
	tmpDir, err := os.MkdirTemp("", "triedandtold-index-*")
	if err != nil {
		log.Fatalf("creating index temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	segPath := filepath.Join(tmpDir, "lexical.seg")
	if err := diskindex.BuildSegment(passages, segPath); err != nil {
		log.Fatalf("building lexical segment: %v", err)
	}
	segment, err := diskindex.OpenSegment(segPath)
	if err != nil {
		log.Fatalf("opening lexical segment: %v", err)
	}

	reverseID := make(map[string]int, len(passages))
	for i, p := range passages {
		reverseID[p.ID()] = i
	}

	deps := api.SearchDeps{
		Lexical:   bm25.WrapSegment(segment),
		Resolver:  segment,
		ReverseID: reverseID,
		Docs:      docs,
		Metas:     metas,
		Params:    bm25.DefaultParams,
	}

	// Semantic search is an enrichment, not a hard dependency - if the
	// embeddings file or the graph build fails, log it and keep serving
	// BM25-only search rather than refusing to start.
	if semIdx, err := buildSemanticIndex(embeddingsPath, filepath.Join(tmpDir, "semantic.graph")); err != nil {
		log.Printf("semantic index unavailable, serving BM25-only search: %v", err)
	} else {
		deps.Semantic = semIdx
		deps.Embedder = embedclient.New(embedServiceURL)
	}

	staticFiles, err := fs.Sub(staticFS, "static")
	if err != nil {
		log.Fatalf("static assets: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /search", api.SearchHandler(deps))
	mux.Handle("GET /", http.FileServer(http.FS(staticFiles)))

	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

func buildSemanticIndex(embeddingsPath, graphPath string) (*semantic.Index, error) {
	embFile, err := embeddings.Open(embeddingsPath)
	if err != nil {
		return nil, err
	}
	idx, err := semantic.Open(graphPath)
	if err != nil {
		return nil, err
	}
	if err := idx.Add(embFile); err != nil {
		return nil, err
	}
	return idx, nil
}

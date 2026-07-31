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
	"triedandtold/internal/extract"
	"triedandtold/internal/index"
	"triedandtold/internal/semantic"
)

//go:embed static
var staticFS embed.FS

const (
	syntheticCorpusPath = "data/synthetic/experiences.jsonl"
	syntheticEmbeddings = "data/embeddings/synthetic.bin"
	realCorpusPath      = "data/real/passages.jsonl"
	realSegmentPath     = "data/real/real.seg"
	realEmbeddings      = "data/embeddings/real.bin"
	embedServiceURL     = "http://127.0.0.1:8091"
)

func main() {
	// The synthetic corpus's lexical segment is rebuilt fresh into a temp
	// dir on every startup - it's small enough that persisting it isn't
	// worth the complexity. The real corpus's segment, by contrast, is a
	// precomputed, committed artifact (built once by cmd/crawl) and is
	// just opened directly - see docs/design/35-combined-corpus.md for
	// why the two corpora are handled differently here.
	syntheticDocs, syntheticMetas, err := corpus.LoadJSONL(syntheticCorpusPath)
	if err != nil {
		log.Fatalf("loading synthetic corpus: %v", err)
	}
	syntheticPassages := corpus.ToPassages(syntheticDocs, syntheticMetas)

	tmpDir, err := os.MkdirTemp("", "triedandtold-index-*")
	if err != nil {
		log.Fatalf("creating index temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	syntheticSegPath := filepath.Join(tmpDir, "synthetic.seg")
	if err := diskindex.BuildSegment(syntheticPassages, syntheticSegPath); err != nil {
		log.Fatalf("building synthetic segment: %v", err)
	}
	syntheticSeg, err := diskindex.OpenSegment(syntheticSegPath)
	if err != nil {
		log.Fatalf("opening synthetic segment: %v", err)
	}

	realDocs, realMetas, err := corpus.LoadRealCrawlJSONL(realCorpusPath)
	if err != nil {
		log.Fatalf("loading real corpus: %v", err)
	}
	realPassages := corpus.ToPassages(realDocs, realMetas)
	realSeg, err := diskindex.OpenSegment(realSegmentPath)
	if err != nil {
		log.Fatalf("opening real segment: %v", err)
	}

	// Docs/Metas are joined to search results purely by stable
	// Passage.ID() (via ReverseID), not by array position - so the two
	// corpora can simply be concatenated here without needing their
	// array indices to line up with anything the segments themselves
	// use internally.
	docs := append(append([]index.IndexDoc{}, syntheticDocs...), realDocs...)
	metas := append(append([]index.DocMeta{}, syntheticMetas...), realMetas...)
	allPassages := append(append([]extract.Passage{}, syntheticPassages...), realPassages...)

	reverseID := make(map[string]int, len(allPassages))
	for i, p := range allPassages {
		reverseID[p.ID()] = i
	}

	multiSeg := diskindex.NewMultiSegment([]*diskindex.Segment{syntheticSeg, realSeg})

	deps := api.SearchDeps{
		Lexical:   bm25.WrapSegment(multiSeg),
		Resolver:  multiSeg,
		ReverseID: reverseID,
		Docs:      docs,
		Metas:     metas,
		Params:    bm25.DefaultParams,
	}

	// Semantic search is an enrichment, not a hard dependency - if either
	// corpus's embeddings fail to load, log it and keep serving
	// BM25-only search rather than refusing to start.
	if semIdx, err := buildSemanticIndex(filepath.Join(tmpDir, "semantic.graph")); err != nil {
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

	log.Printf("serving %d synthetic + %d real passages", len(syntheticDocs), len(realDocs))
	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

func buildSemanticIndex(graphPath string) (*semantic.Index, error) {
	syntheticEmb, err := embeddings.Open(syntheticEmbeddings)
	if err != nil {
		return nil, err
	}
	realEmb, err := embeddings.Open(realEmbeddings)
	if err != nil {
		return nil, err
	}

	idx, err := semantic.Open(graphPath)
	if err != nil {
		return nil, err
	}
	if err := idx.Add(syntheticEmb); err != nil {
		return nil, err
	}
	if err := idx.Add(realEmb); err != nil {
		return nil, err
	}
	return idx, nil
}

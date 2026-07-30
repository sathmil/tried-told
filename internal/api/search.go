// Package api exposes the search index over HTTP.
package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"triedandtold/internal/bm25"
	"triedandtold/internal/hybrid"
	"triedandtold/internal/index"
	"triedandtold/internal/semantic"
)

// semanticResultLimit caps how many semantic nearest-neighbors are pulled
// into the fused ranking - a top-N bound on the ANN search itself, not a
// relevance cutoff.
const semanticResultLimit = 10

type searchResult struct {
	DocID    int    `json:"doc_id"`
	Rank     int    `json:"rank"`
	Text     string `json:"text"`
	Source   string `json:"source"`
	Product  string `json:"product"`
	SkinTone string `json:"skin_tone"`
	Climate  string `json:"climate"`
}

// Embedder is the query-embedding half of hybrid search, satisfied by
// *embedclient.Client. Kept as an interface so tests can fake it without
// a real running embed_service.py.
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

// SearchDeps bundles everything SearchHandler needs. Lexical is
// segment-backed (not the plain in-memory index.Index) specifically so
// Resolver can map a BM25 hit's local DocID to a stable Passage.ID() -
// the join key hybrid ranking needs to merge lexical and semantic
// results (see docs/design/29-hybrid-search-api.md).
type SearchDeps struct {
	Lexical  bm25.Index
	Resolver bm25.PassageIDResolver

	// ReverseID maps Passage.ID() back to the local DocID used to index
	// Docs/Metas, so fused results (which are Passage IDs) can still be
	// rendered with each document's text and metadata.
	ReverseID map[string]int

	// Semantic and Embedder are both nil to run BM25-only. Both must be
	// set to enable hybrid ranking.
	Semantic *semantic.Index
	Embedder Embedder

	Docs   []index.IndexDoc
	Metas  []index.DocMeta
	Params bm25.Params
}

// SearchHandler serves GET /search?q=.... With Semantic and Embedder
// configured, results are ranked by Reciprocal Rank Fusion of BM25 and
// semantic search (docs/design/28-rrf.md); otherwise, or if the embedding
// service call fails for this request, it falls back to BM25 alone -
// semantic search is an enrichment on top of lexical search, not a hard
// dependency of it, so its unavailability degrades ranking quality rather
// than failing the request.
func SearchHandler(deps SearchDeps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		if q == "" {
			http.Error(w, `missing required query parameter "q"`, http.StatusBadRequest)
			return
		}

		bm25Hits := bm25.Search(deps.Lexical, q, deps.Params)
		lexicalRanking := make([]string, len(bm25Hits))
		for i, h := range bm25Hits {
			lexicalRanking[i] = deps.Resolver.PassageID(h.DocID)
		}

		fusedIDs := lexicalRanking
		if deps.Embedder != nil && deps.Semantic != nil {
			vec, err := deps.Embedder.Embed(r.Context(), q)
			if err != nil {
				log.Printf("api: semantic search unavailable, falling back to BM25-only: %v", err)
			} else {
				semanticRanking := deps.Semantic.Search(vec, semanticResultLimit)
				fusedIDs = hybrid.Fuse([][]string{lexicalRanking, semanticRanking}, hybrid.DefaultK)
			}
		}

		results := make([]searchResult, 0, len(fusedIDs))
		for i, pid := range fusedIDs {
			docID, ok := deps.ReverseID[pid]
			if !ok {
				continue
			}
			m := deps.Metas[docID]
			results = append(results, searchResult{
				DocID:    docID,
				Rank:     i + 1,
				Text:     deps.Docs[docID].Text,
				Source:   m.Source,
				Product:  m.Product,
				SkinTone: m.SkinTone,
				Climate:  m.Climate,
			})
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(results)
	}
}

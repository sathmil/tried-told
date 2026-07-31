// Package api exposes the search index over HTTP.
package api

import (
	"context"
	"encoding/json"
	"html"
	"log"
	"net/http"
	"strings"

	"triedandtold/internal/bm25"
	"triedandtold/internal/hybrid"
	"triedandtold/internal/index"
	"triedandtold/internal/semantic"
	"triedandtold/internal/snippet"
)

// semanticResultLimit caps how many semantic nearest-neighbors are pulled
// into the fused ranking - a top-N bound on the ANN search itself, not a
// relevance cutoff.
const semanticResultLimit = 10

type searchResult struct {
	DocID int `json:"doc_id"`
	Rank  int `json:"rank"`

	// SnippetHTML is pre-rendered, already-escaped HTML (plain text with
	// matched query terms wrapped in <mark>) - not the raw passage text.
	// Named explicitly rather than "text" so nothing downstream mistakes
	// it for plain text and double-escapes or misrenders it. Built here,
	// not client-side, specifically to avoid ever sending byte offsets
	// (correct for Go's UTF-8 strings) to a JS frontend (which indexes
	// strings in UTF-16 code units) - real crawled content has multi-byte
	// and even surrogate-pair characters (design doc 34), so that
	// cross-language offset translation is a real bug class, not a
	// theoretical one, and the simplest way to avoid it is to never do it.
	SnippetHTML string `json:"snippet_html"`

	Source   string `json:"source"`
	Product  string `json:"product"`
	SkinTone string `json:"skin_tone"`
	Climate  string `json:"climate"`
}

// renderSnippetHTML converts a snippet.Snippet's plain text + byte-offset
// matches into a single escaped HTML string, wrapping each match in
// <mark>...</mark>. s.Matches is assumed non-overlapping and in
// left-to-right order, which is how snippet.Extract always produces them.
func renderSnippetHTML(s snippet.Snippet) string {
	var b strings.Builder
	pos := 0
	for _, m := range s.Matches {
		b.WriteString(html.EscapeString(s.Text[pos:m.Start]))
		b.WriteString("<mark>")
		b.WriteString(html.EscapeString(s.Text[m.Start:m.End]))
		b.WriteString("</mark>")
		pos = m.End
	}
	b.WriteString(html.EscapeString(s.Text[pos:]))
	return b.String()
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
			snip := snippet.Extract(deps.Docs[docID].Text, q)
			results = append(results, searchResult{
				DocID:       docID,
				Rank:        i + 1,
				SnippetHTML: renderSnippetHTML(snip),
				Source:      m.Source,
				Product:     m.Product,
				SkinTone:    m.SkinTone,
				Climate:     m.Climate,
			})
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(results)
	}
}

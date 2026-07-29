// Package api exposes the search index over HTTP.
package api

import (
	"encoding/json"
	"net/http"

	"triedandtold/internal/bm25"
	"triedandtold/internal/index"
)

type searchResult struct {
	DocID    int     `json:"doc_id"`
	Score    float64 `json:"score"`
	Text     string  `json:"text"`
	Source   string  `json:"source"`
	Product  string  `json:"product"`
	SkinTone string  `json:"skin_tone"`
	Climate  string  `json:"climate"`
}

// SearchHandler serves GET /search?q=... against idx, rendering each hit's
// text and metadata from docs/metas (both indexed by DocID).
func SearchHandler(idx index.Index, docs []index.IndexDoc, metas []index.DocMeta, params bm25.Params) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("q")
		if q == "" {
			http.Error(w, `missing required query parameter "q"`, http.StatusBadRequest)
			return
		}

		hits := bm25.Search(idx, q, params)
		results := make([]searchResult, len(hits))
		for i, h := range hits {
			m := metas[h.DocID]
			results[i] = searchResult{
				DocID:    h.DocID,
				Score:    h.Score,
				Text:     docs[h.DocID].Text,
				Source:   m.Source,
				Product:  m.Product,
				SkinTone: m.SkinTone,
				Climate:  m.Climate,
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(results)
	}
}

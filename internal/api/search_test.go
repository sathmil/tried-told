package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"triedandtold/internal/bm25"
	"triedandtold/internal/index"
)

func setup() (index.Index, []index.IndexDoc, []index.DocMeta) {
	docs := []index.IndexDoc{
		{ID: 0, Text: "sunscreen leaves white cast"},
		{ID: 1, Text: "moisturizer is great"},
	}
	metas := []index.DocMeta{
		{ID: 0, Source: "synthetic", Product: "sunscreen", SkinTone: "deep"},
		{ID: 1, Source: "synthetic", Product: "moisturizer"},
	}
	return index.BuildIndex(docs), docs, metas
}

func TestSearchHandler_MissingQueryReturns400(t *testing.T) {
	idx, docs, metas := setup()
	handler := SearchHandler(idx, docs, metas, bm25.DefaultParams)

	req := httptest.NewRequest(http.MethodGet, "/search", nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestSearchHandler_ReturnsMatchingResults(t *testing.T) {
	idx, docs, metas := setup()
	handler := SearchHandler(idx, docs, metas, bm25.DefaultParams)

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
	idx, docs, metas := setup()
	handler := SearchHandler(idx, docs, metas, bm25.DefaultParams)

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

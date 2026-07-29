// Command server runs the Tried & Told search HTTP service.
package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"

	"triedandtold/internal/api"
	"triedandtold/internal/bm25"
	"triedandtold/internal/corpus"
	"triedandtold/internal/index"
)

//go:embed static
var staticFS embed.FS

func main() {
	docs, metas, err := corpus.LoadJSONL("data/synthetic/experiences.jsonl")
	if err != nil {
		log.Fatalf("loading corpus: %v", err)
	}
	idx := index.BuildIndex(docs)

	staticFiles, err := fs.Sub(staticFS, "static")
	if err != nil {
		log.Fatalf("static assets: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /search", api.SearchHandler(idx, docs, metas, bm25.DefaultParams))
	mux.Handle("GET /", http.FileServer(http.FS(staticFiles)))

	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

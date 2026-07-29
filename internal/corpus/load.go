// Package corpus loads documents from disk into the types index.BuildIndex
// expects, per docs/design/03-initial-corpus-sourcing.md.
package corpus

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"

	"triedandtold/internal/index"
)

type record struct {
	Text     string `json:"text"`
	Source   string `json:"source"`
	Product  string `json:"product"`
	SkinTone string `json:"skin_tone"`
	Climate  string `json:"climate"`
}

// LoadJSONL reads one JSON object per line from path. IDs are assigned by
// line order, starting at 0 — consistent with BuildIndex's ascending-ID
// requirement.
func LoadJSONL(path string) ([]index.IndexDoc, []index.DocMeta, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	var docs []index.IndexDoc
	var metas []index.DocMeta

	scanner := bufio.NewScanner(f)
	id := 0
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var rec record
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			return nil, nil, fmt.Errorf("corpus.LoadJSONL: line %d: %w", id+1, err)
		}
		docs = append(docs, index.IndexDoc{ID: id, Text: rec.Text})
		metas = append(metas, index.DocMeta{
			ID:       id,
			Source:   rec.Source,
			Product:  rec.Product,
			SkinTone: rec.SkinTone,
			Climate:  rec.Climate,
		})
		id++
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, err
	}

	return docs, metas, nil
}

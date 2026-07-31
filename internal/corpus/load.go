// Package corpus loads documents from disk into the types index.BuildIndex
// expects, per docs/design/03-initial-corpus-sourcing.md.
package corpus

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"

	"triedandtold/internal/extract"
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

// realRecord is cmd/crawl's output schema (data/real/passages.jsonl) -
// deliberately narrower than record: a real crawled page has no
// structured product/skin_tone/climate markup to report, so there's no
// field to read one from.
type realRecord struct {
	Text      string `json:"text"`
	SourceURL string `json:"source_url"`
}

// LoadRealCrawlJSONL reads cmd/crawl's output format - one real,
// extracted passage per line, already keyed by the SourceURL its
// Passage.ID() was computed from (see docs/design/33-real-crawl.md).
// IDs are assigned by line order, matching the local-ID order
// diskindex.BuildSegment assigned when cmd/crawl built the committed
// segment from this same file - so ToPassages(these docs, these metas)
// reproduces identical Passage.ID()s to what's already in that segment.
func LoadRealCrawlJSONL(path string) ([]index.IndexDoc, []index.DocMeta, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()

	var docs []index.IndexDoc
	var metas []index.DocMeta

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	id := 0
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var rec realRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			return nil, nil, fmt.Errorf("corpus.LoadRealCrawlJSONL: line %d: %w", id+1, err)
		}
		docs = append(docs, index.IndexDoc{ID: id, Text: rec.Text})
		metas = append(metas, index.DocMeta{ID: id, Source: rec.SourceURL})
		id++
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, err
	}

	return docs, metas, nil
}

// ToPassages converts already-loaded docs/metas back into
// []extract.Passage, in the same order, so a segment built from them
// assigns local IDs that line up index-for-index with docs and metas.
// The synthetic corpus has no real crawl source, so "synthetic" (this
// dataset's fixed rec.Source value) stands in for SourceURL -
// extract.Passage.ID() only needs it to be stable and to differ when the
// text does, not to be a resolvable URL.
func ToPassages(docs []index.IndexDoc, metas []index.DocMeta) []extract.Passage {
	passages := make([]extract.Passage, len(docs))
	for i, d := range docs {
		m := metas[i]
		passages[i] = extract.Passage{
			Text:      d.Text,
			SourceURL: m.Source,
			Product:   m.Product,
			SkinTone:  m.SkinTone,
			Climate:   m.Climate,
		}
	}
	return passages
}

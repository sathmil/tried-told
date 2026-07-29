# Design Decision 01: Document Representation & In-Memory Inverted Index

Status: decided, Milestone 1
Date: 2026-07-27

## Decision

**Document representation:** split into two parallel structs, both keyed by the same
sequential int ID.

```go
type IndexDoc struct {
    ID   int
    Text string
}

type DocMeta struct {
    ID      int
    Source  string
    Product string
    // additional display/filter fields as needed
}
```

`IndexDoc` holds only what the tokenizer/index needs to touch. `DocMeta` holds what
search results need to render. Both live in slices indexed by ID (`indexDocs[id]`,
`docMeta[id]`), so lookup by ID is O(1) either way.

**Inverted index:** map of term to a slice of postings, sorted by DocID.

```go
type Posting struct {
    DocID int
    Freq  int
}

type Index map[string][]Posting // postings sorted ascending by DocID
```

## Alternatives considered

- **String IDs (e.g. source URL or hash) instead of sequential ints.** Rejected for
  now: sequential ints are dense, so they double as direct array indices (O(1)
  access into `indexDocs`/`docMeta`) and are far cheaper to store repeatedly inside
  postings lists. Tradeoff noted: these IDs are only stable for one batch load —
  once incremental re-crawling exists, "doc #47" can't just mean "the 47th thing
  loaded." Revisit at the disk-backed index milestone.
- **`map[string][]int` postings (docID only, no frequency).** Rejected: cannot
  support BM25, which requires term frequency per document.
- **`map[string]map[int]int` postings (term -> docID -> freq).** Rejected in favor
  of the sorted slice. A map gives O(1) lookup for one *known* docID, but that is
  not BM25's access pattern — BM25 always walks the *entire* postings list for a
  term to accumulate scores, never looks up a single specific ID. For that
  sequential-scan pattern, a sorted slice of structs wins on:
  - cache locality (contiguous memory vs. scattered hash buckets)
  - memory overhead (struct slice vs. Go map bucket overhead, matters at scale)
  - determinism (Go randomizes map iteration order on purpose, which fights the
    requirement for deterministic ranking tests)
  - enabling merge-join intersection for AND/phrase queries later, which requires
    sorted order on both sides

## Why the docID must be kept, not just the frequency

A real query has multiple terms (e.g. "sunscreen" AND "humid"). Each term's
postings list is looked up independently, and per-term contributions are
accumulated into a running score keyed by docID
(`scores[posting.DocID] += bm25Contribution(posting.Freq, ...)`). Without the
docID attached to each posting, a frequency value can't be tied back to a specific
document when merging contributions across terms.

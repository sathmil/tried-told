# Tried & Told

A vertical search engine for first-person skincare and makeup
experiences, built from scratch in Go to learn information retrieval
deeply enough to defend every architectural decision — not a wrapper
around Elasticsearch or a vector-database SDK.

Search combines classic lexical retrieval (BM25, over a
delta/varint-compressed disk index built by hand) with dense semantic
retrieval (HNSW over sentence embeddings), fused with Reciprocal Rank
Fusion. Content is sourced by a real, polite, crash-resumable web
crawler running against individually vetted sites, not a synthetic or
scraped-without-permission dataset.

## Highlights

- **BM25 and an inverted index implemented from scratch** — no search
  library. Delta + variable-byte encoded postings, phrase search via
  position intersection, tombstone-based soft deletes, and segment
  merging that actually reclaims the space deletions free up.
- **A real crawler with real ethics.** Every source was vetted by
  fetching and reading its `robots.txt` directly and checking terms of
  service for scraping restrictions — one platform was declined outright
  for prohibiting it. The crawler enforces politeness (per-host rate
  limiting), identifies itself with a real User-Agent, and recovers from
  crashes via a write-ahead log rather than starting over.
- **Hybrid semantic + lexical ranking.** Passage embeddings
  (`bge-small-en-v1.5`) indexed with HNSW, combined with BM25 via
  Reciprocal Rank Fusion — chosen specifically because BM25 and cosine
  similarity scores live on incomparable scales, and RRF sidesteps that
  by ranking on position, not raw score.
- **Real bugs, found and fixed, not just features shipped.** Two
  independently reproduced bugs in a third-party HNSW library, a
  Unicode-normalization gap that made decorative styled text
  unsearchable, and a UTF-8/UTF-16 offset mismatch that would have
  broken snippet highlighting on real content — each one found through
  testing against real crawled data, not synthetic fixtures.
- **835 real, ethically sourced passages** from vetted sites, indexed
  alongside a small synthetic demo set, all searchable through one
  combined index.
- **163 automated tests** and a running design-decision log
  (`docs/design/`, `docs/LEARNING.md`) documenting the reasoning behind
  every major choice — including the ones that turned out to be wrong
  and had to be revisited.

## Architecture

| Component | What it does |
|---|---|
| `internal/fetch`, `internal/frontier`, `internal/robots` | Polite, crash-resumable crawling: per-host rate limiting, robots.txt enforcement, retry with backoff |
| `internal/crawlstate`, `internal/wal` | Write-ahead-log-backed frontier/registry state, so a crash resumes instead of restarting |
| `internal/dedup`, `internal/simhash` | Bloom-filter exact dedup + SimHash near-duplicate detection |
| `internal/extract` | Site-specific passage extraction (deliberately not generic boilerplate stripping) |
| `internal/tokenize`, `internal/bm25`, `internal/index` | Tokenization (with Unicode compatibility normalization), BM25 scoring, in-memory index |
| `internal/diskindex` | Immutable, compressed disk segments: postings, phrase search, multi-segment querying, merging |
| `internal/embeddings`, `internal/semantic`, `internal/embedclient` | Embedding file format, HNSW index, query-time embedding client |
| `internal/hybrid` | Reciprocal Rank Fusion of lexical and semantic rankings |
| `internal/snippet` | Query-highlighted excerpt extraction |
| `internal/api`, `cmd/server` | The HTTP search API and web UI |
| `cmd/crawl` | The real, vetted crawl that sourced the live corpus |
| `python/` | Offline embedding generation only — Go owns everything else, including the vector index |

Every non-obvious decision — why BM25's `k1`/`b` were chosen, why
segments are immutable, why RRF over a weighted score combination, why
crawl link-discovery was bounded the way it was — is written up in
[`docs/design/`](docs/design), in commit order, with the rejected
alternatives and why they lost.

## Running it

```bash
# 1. Start the query-time embedding service (optional - search falls
#    back to lexical-only if this isn't running)
cd python
python3 -m venv .venv && source .venv/bin/activate
pip install -r requirements.txt
python3 embed_service.py 8091

# 2. In another terminal, start the search server
go run ./cmd/server
# -> http://localhost:8080
```

```bash
go test ./...   # run the test suite
```

## Stack

Go (crawler, indexing, ranking, query API) · Python (offline embedding
generation only) · no external search or vector database — the index is
a hand-built, disk-backed file format.

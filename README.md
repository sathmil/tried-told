# Tried & Told

A full-text search engine, built from scratch, for discovering
first-person product reviews across the web — no off-the-shelf search
library or vector database. Indexing, ranking, and storage are all
implemented from first principles in Go, as a deliberate deep dive into
information retrieval: every architectural decision below is one I can
defend, not one a framework made for me.

## Why

Product reviews on the open web are scattered, duplicated, and often
not written by an actual person who used the product. Tried & Told is a
vertical search engine focused specifically on surfacing authentic,
first-person reviews — sourced, deduplicated, and ranked for relevance
rather than SEO.

### Why I built this

I'd used vector databases and retrieval tools before, but I understood how to call them a lot better than I understood what was happening underneath. Tried & Told was how I closed that gap — building the pieces I'd normally get from a library myself, against real first-person product reviews instead of a synthetic demo set.

Getting the data ended up being one of the first things that complicated the project. I initially found much larger datasets that would have made this easier, but some had licensing I wasn't comfortable ignoring, and sites differed in what they actually permitted through `robots.txt` and their terms. I rejected sources I couldn't confidently use and built around a much smaller set of vetted public sources instead — which made it clear retrieval starts before ranking: what gets included in the corpus decides whose information can be found at all.

From there I mostly let whatever I didn't understand yet decide what to build next. BM25 made me figure out what `k1` and `b` actually change. Putting the index on disk led to compressed postings, segments, tombstones, and merging. Adding semantic search meant confronting that BM25 and cosine-similarity scores don't mean the same thing, which led to RRF. Real crawled data then broke assumptions a toy example never would have: Unicode that looked completely normal but couldn't be matched by a plain-text query, and edge cases in a third-party HNSW library that a clean demo never would have hit.

I used Claude throughout this — mainly to reason through tradeoffs and move quickly past syntax, while I made the actual calls and dug into whatever broke once it was built.

## Highlights

- **BM25 and an inverted index implemented from scratch.** Delta +
  variable-byte encoded postings, phrase search via position
  intersection, tombstone-based soft deletes, and segment merging that
  actually reclaims the space deletions free up.
- **A real crawler with real ethics.** Every source was vetted by
  fetching and reading its `robots.txt` directly and checking terms of
  service for scraping restrictions — one platform was declined outright
  for prohibiting it. The crawler enforces politeness (per-host rate
  limiting), identifies itself with a real User-Agent, and recovers from
  crashes via a write-ahead log rather than starting over.
- **Hybrid semantic + lexical ranking**, fused with Reciprocal Rank
  Fusion rather than a weighted score blend — chosen specifically
  because BM25 and cosine-similarity scores live on incomparable scales,
  and RRF sidesteps that by ranking on position, not raw score.
- **Real bugs, found and fixed, not just features shipped.** Two
  independently reproduced bugs in a third-party HNSW library, a
  Unicode-normalization gap that made decorative styled text
  unsearchable, and a UTF-8/UTF-16 offset mismatch that would have
  broken snippet highlighting on real content — each found through
  testing against real crawled data, not synthetic fixtures.
- **835 first-person reviews**, ethically sourced and vetted from 45
  pages, filtered for authenticity via SimHash near-duplicate detection,
  indexed alongside a small synthetic demo set through one combined
  index.
- **163 automated tests** and a running design-decision log
  (`docs/design/`, `docs/LEARNING.md`) documenting the reasoning behind
  every major choice — including the ones that turned out wrong and had
  to be revisited.

## Features

**Indexing**
- Disk-backed inverted index with delta/varint-compressed postings for
  compact storage
- Positional indexing for exact phrase search
- Immutable segments with soft-delete and merge-time garbage collection

**Crawling**
- Polite, crash-resumable crawler with `robots.txt` enforcement and
  per-host rate limiting
- Write-ahead-log (WAL) recovery so crawls resume cleanly after a
  failure
- Bloom-filter URL dedup plus SimHash near-duplicate detection for
  re-published or lightly-edited content

**Retrieval & Ranking**
- BM25 over the lexical index
- Hybrid retrieval: BM25 fused with HNSW approximate-nearest-neighbor
  search over dense embeddings
- Reciprocal Rank Fusion (RRF) to combine lexical and semantic result
  sets
- Offline Python embedding pipeline feeding a Go-based query API

## Architecture

```
┌──────────────┐     ┌──────────────┐     ┌─────────────────┐
│   Crawler    │ --> │   Indexer    │ --> │  Inverted Index  │
│ (WAL, dedup) │     │ (BM25 terms) │     │  (disk-backed)   │
└──────────────┘     └──────────────┘     └─────────────────┘
                                                     │
┌──────────────┐     ┌──────────────┐               │
│  Embedding   │ --> │  HNSW Index  │               │
│  Pipeline    │     │ (dense vecs) │               │
│  (Python)    │     └──────────────┘               │
└──────────────┘             │                       │
                              ▼                       ▼
                        ┌──────────────────────────────┐
                        │   Search API (Go) — RRF       │
                        │   fusing lexical + semantic   │
                        └──────────────────────────────┘
```

| Package | What it does |
|---|---|
| `internal/fetch`, `internal/frontier`, `internal/robots` | Polite, crash-resumable crawling: per-host rate limiting, robots.txt enforcement, retry with backoff |
| `internal/crawlstate`, `internal/wal` | Write-ahead-log-backed frontier/registry state |
| `internal/dedup`, `internal/simhash` | Bloom-filter exact dedup + SimHash near-duplicate detection |
| `internal/extract` | Site-specific passage extraction (deliberately not generic boilerplate stripping) |
| `internal/tokenize`, `internal/bm25`, `internal/index` | Tokenization (with Unicode compatibility normalization), BM25 scoring, in-memory index |
| `internal/diskindex` | Compressed disk segments: postings, phrase search, multi-segment querying, merging |
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
alternatives and why they lost. [`docs/LEARNING.md`](docs/LEARNING.md)
is a running log of what each step taught, including mistakes caught
along the way.

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

## Tech Stack

**Go** — search API, inverted index, crawler, ranking · **Python** —
offline embedding pipeline only, per a deliberate stack boundary · no
external search or vector database — the index is a hand-built,
disk-backed file format.

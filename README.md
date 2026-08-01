# Tried & Told

A full-text search engine, built from scratch, for discovering first-person product reviews across the web.

Tried & Told crawls, indexes, and ranks first-person product review content using a custom-built inverted index, a crash-resumable crawler, and a hybrid lexical + semantic retrieval pipeline. No off-the-shelf search library — indexing, ranking, and storage are all implemented from first principles.

## Why

Product reviews on the open web are scattered, duplicated, and often not written by an actual person who used the product. Tried & Told is a vertical search engine focused specifically on surfacing authentic, first-person reviews — sourced, deduplicated, and ranked for relevance rather than SEO.

## Features

**Indexing**
- Disk-backed inverted index with delta/varint-compressed postings lists for compact storage
- Positional indexing to support exact phrase search
- Segment-based storage with soft-delete and background garbage collection

**Crawling**
- Polite, crash-resumable crawler with `robots.txt` enforcement and per-host rate limiting
- Write-ahead-log (WAL) recovery so crawls can resume cleanly after a failure
- Bloom-filter based URL deduplication
- SimHash near-duplicate detection to filter re-published or lightly-edited content

**Retrieval & Ranking**
- BM25 ranking over the lexical index
- Hybrid retrieval: BM25 results fused with an HNSW approximate-nearest-neighbor search over dense embeddings
- Reciprocal Rank Fusion (RRF) to combine lexical and semantic result sets
- Offline Python embedding pipeline feeding a Go-based search API

## Architecture

```
┌─────────────┐     ┌──────────────┐     ┌────────────────┐
│   Crawler    │ --> │   Indexer    │ --> │  Inverted Index │
│  (WAL, dedup)│     │ (BM25 terms) │     │  (disk-backed)  │
└─────────────┘     └──────────────┘     └────────────────┘
                                                    │
┌─────────────┐     ┌──────────────┐               │
│  Embedding   │ --> │  HNSW Index   │              │
│  Pipeline    │     │  (dense vecs) │              │
│  (Python)    │     └──────────────┘               │
└─────────────┘             │                       │
                             ▼                       ▼
                        ┌─────────────────────────────┐
                        │   Search API (Go) — RRF      │
                        │   fusing lexical + semantic   │
                        └─────────────────────────────┘
```

## Tech Stack

- **Go** — search API, inverted index, crawler
- **Python** — offline embedding pipeline
- **BM25**, **HNSW**, **Reciprocal Rank Fusion** — retrieval and ranking

## Dataset

835 first-person reviews ethically sourced and vetted from 45 pages, filtered for authenticity via SimHash near-duplicate detection.

## Testing

163 automated tests covering indexing, crash recovery, and retrieval correctness. See `DESIGN.md` for a running log of architectural tradeoffs and library defects identified and repaired during development.

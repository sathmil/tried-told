# Design Decision 27: Query-Time Embedding Service

Status: decided, Semantic indexing (in progress)
Date: 2026-07-30

## Decision

A live user query is embedded by `python/embed_service.py`, a small
long-lived process (the `bge-small-en-v1.5` model loads once, at startup)
that exposes a single `POST /embed` endpoint over plain HTTP+JSON.
`internal/embedclient.Client` is the Go-side caller.

This resolves the open question left at the end of design doc 26: the
stack mandate says Go owns the query service, but embedding a query needs
the exact same PyTorch model that generated the passage embeddings
(design doc 25). Two options existed - a Python microservice, or exporting
the model to ONNX for a pure-Go query-time path. Chosen: the microservice,
because it reuses the literal same `sentence-transformers` code path that
produced the passage embeddings, so there's no fidelity risk (no export
step, no reimplementing BGE's WordPiece tokenizer in Go). The cost -
a second running process, and a network hop on the query path - is
accepted for now; latency isn't the current concern, correctness is.

## Transport: HTTP+JSON, not gRPC

The payload in both directions is tiny (one short string in, 384 floats
out), so gRPC's main advantage - efficient serialization for large or
high-throughput payloads - doesn't apply here. HTTP+JSON needs no codegen
step and is debuggable with plain `curl`, which mattered directly during
development (see verification below).

## The prefix lives in the service, not the caller

BGE's asymmetric encoding means queries need an instruction prefix
("Represent this sentence for searching relevant passages: ") that
passages must never get (`generate_embeddings.py` deliberately never adds
it). `embed_service.py` applies that prefix internally, since this
endpoint's entire purpose is embedding queries - callers send raw query
text and get back a ready-to-compare vector, without needing to know
BGE-specific encoding conventions. That convention is a model-specific
detail and stays encapsulated where the model lives.

## Verification: a real live round trip, not just a mocked contract

`internal/embedclient/client_test.go` covers the client's own logic
(success, HTTP error, context cancellation) against an `httptest` server -
useful for fast, hermetic unit coverage, but it only proves the Go code
parses whatever JSON shape the test hands it, not that the real service
actually speaks that shape.

So before trusting the design, the real `embed_service.py` was started
and hit twice, independently:

1. Directly via `curl`, decoding the response and confirming: 384
   dimensions, L2 norm ≈ 1.0 (i.e. `normalize_embeddings=True` really
   took effect over HTTP, not just in the offline script), and that
   `/embed` on empty/missing `text` returns 400 rather than crashing.
2. Via the real `embedclient.Client` (a throwaway `cmd/embedclientcheck`
   program, deleted after use, same pattern as prior throwaway
   verification scripts this project) - confirming the actual Go client
   and the actual Python service agree on the wire format, not just that
   each side matches its own test's assumptions.

Both returned a 384-dim, unit-norm vector, matching the passage embeddings'
convention exactly.

## Deliberately out of scope for this pass

- **Process lifecycle**: how `embed_service.py` gets started/supervised
  in a real deployment (systemd unit, container, etc.) - not decided, not
  needed to prove the architecture works.
- **Wiring into the actual query API**: `internal/api` doesn't call
  `embedclient` yet. This pass proves the embedding call works
  end-to-end; using it inside a real search request, and combining its
  results with BM25 (hybrid ranking, e.g. RRF), is later work.
- Tombstone-awareness for semantic search results (matching
  `bm25.FilterDeleted`) - still open, as noted in design doc 26.

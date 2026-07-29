# Design Decision 03: Initial Corpus Sourcing (Milestone 1)

Status: decided, Milestone 1
Date: 2026-07-28

## Decision

Use a small (~30 document) hand-authored **synthetic** dataset to prove the
pipeline end-to-end, rather than real scraped/licensed content, for this first
checkpoint only.

- Location: `data/synthetic/experiences.jsonl` (one JSON object per line).
- Every document has `"source": "synthetic"` so it can never be confused with
  or displayed as real user data.
- Fictional/generic product names only — never a real brand — since these are
  fabricated entries and attaching them to a real product would look like a
  fabricated review of a real product.
- No `id` field in the file: doc IDs are assigned by the loader based on line
  order at load time, consistent with the sequential-int-ID decision in
  design doc 01.
- Deliberately covers the representation dimensions this project cares about:
  deep/olive/South Asian/medium-deep/fair skin tones, humid tropical and
  dry/cold climates, both positive and negative experiences, and negation
  phrasing (`"doesn't crease"`, `"didn't leave a white cast"`) that stresses
  the tokenizer decisions from design doc 02.

## Source documentation (per corpus policy)

- **License/permission:** N/A — self-authored, not derived from any real
  review or third party text.
- **robots.txt / ToS:** N/A — no crawling involved.
- **Excerpting/display:** Fine to display in full; it's fabricated test data.
- **Attribution:** None required; labeled `source: synthetic` instead.
- **Deletion requirements:** None; not personal data, nothing to honor a
  deletion request for.

## Why not real data yet

This checkpoint's goal (per the project plan) is to prove the pipeline
mechanics work, not to produce a meaningful retrieval-quality evaluation —
that comes at the 5,000–10,000 doc checkpoint. Sourcing real, legally usable
data (a properly licensed research dataset, or opt-in Stackd/Thread
submissions) requires real due-diligence work that shouldn't block finishing
the first vertical slice. **This synthetic set does not count toward the
500–1,000 document corpus target** — it is a disposable test fixture, not
early corpus growth. Real sourcing is separate future work (task in progress
tracker).

## Alternatives considered

- **Existing openly-licensed review dataset.** Rejected for now: verifying an
  actual redistribution-permitting license is real research work, not
  something to rush to unblock a pipeline test.
- **Real Stackd/Thread opt-in data.** Not yet available/confirmed; revisit
  once it exists.

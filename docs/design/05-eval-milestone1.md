# Design Decision 05: Initial Evaluation Queries (Milestone 1)

Status: decided, Milestone 1
Date: 2026-07-29

## Decision

10 hand-judged queries against the synthetic corpus, stored as **sparse
qrels** (`data/eval/queries.json`): each query lists only the documents judged
relevant (`relevance: 1` or `2`); any document not listed is implicitly
irrelevant (`0`).

- Rejected exhaustive/dense judging (labeling all 30 docs per query) because
  it doesn't scale — judging every document against every query is only
  feasible at this toy corpus size. At 50,000+ docs it's infeasible, so the
  workflow needs to already be the one that scales: sparse judgments, same
  as TREC-style qrels in real IR evaluation.
- Metrics computed now: **Precision@10** and **MRR** — meaningful even with
  sparse judgments, and enough to sanity-check the pipeline per the "no
  quality claim without a reproducible measurement" rule.
- Deferred: **Recall@10** (needs exhaustive judgments to mean anything — with
  sparse qrels we don't know the true total relevant count) and **nDCG@10**
  (more machinery than this checkpoint needs). Both belong to the fuller
  50-200 query evaluation harness at the next corpus checkpoint.

## Query design

Covers the categories the full evaluation plan calls for, scaled down to 10:
representation/undertone-specific (skin tone, South Asian, climate),
experience-specific with negation, broad informational, and one deliberate
**difficult paraphrase** (query 7: describes oxidation without ever using the
word "oxidize"). That last one is expected to score poorly under pure BM25 —
it's a planted example of the exact vocabulary-mismatch failure mode that
motivates adding semantic retrieval later, not a bug to fix now.

## Metric definitions

- `PrecisionAtK` = (# relevant docs in top `k` results) / `k`, treating unfilled
  ranks (fewer than `k` results returned at all) as non-relevant.
- `ReciprocalRank` = `1 / rank` of the first relevant result in the ranked
  list; `0` if no relevant result appears at all.

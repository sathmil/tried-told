# Milestone 1 Evaluation Results

Reproduce with:
```
go run ./cmd/eval
```

Corpus: `data/synthetic/experiences.jsonl` (30 synthetic docs). Queries:
`data/eval/queries.json` (10 hand-judged, sparse qrels — see
docs/design/05-eval-milestone1.md).

| Query | P@10 | RR |
|---|---|---|
| sunscreen that doesn't leave a white cast on deep skin in humid weather | 0.20 | 1.00 |
| foundation for olive undertones that doesn't oxidize | 0.40 | 1.00 |
| moisturizer experiences from people with oily, sensitive skin | 0.40 | 1.00 |
| concealer that didn't crease on dry under-eyes | 0.40 | 1.00 |
| best sunscreen for oily skin that doesn't feel greasy | 0.10 | 1.00 |
| products that hold up in humid tropical climates without feeling heavy | 0.40 | 1.00 |
| makeup that stays true to color all day without turning weird colors (paraphrase stress-test) | 0.50 | 1.00 |
| concealer that creases and settles into fine lines | 0.20 | 1.00 |
| South Asian skin tone product matches | 0.20 | 1.00 |
| moisturizer that doesn't clog pores for acne-prone skin | 0.20 | 1.00 |

**Mean Precision@10: 0.300** · **MRR: 1.000**

## Interpretation

- **MRR = 1.0 is the headline result**: for every query, the single
  top-ranked document was judged relevant. Basic BM25 retrieval is working
  correctly end-to-end.
- **Precision@10 numbers are not directly comparable to future milestones.**
  Several queries have fewer than 10 judged-relevant documents in the entire
  30-doc corpus (e.g. query 5 has only 3), which mechanically caps P@10 well
  below 1.0 even for perfect ranking. At this corpus size, P@10 mostly
  reflects "how many relevant docs exist," not ranking quality — a limitation
  of applying a top-10 metric to a 30-document corpus, not a retrieval
  failure. Re-evaluate this metric's usefulness once the corpus reaches the
  5,000-10,000 doc checkpoint, where relevant sets per query are large enough
  for it to mean something.
- **The planted paraphrase query (query 7) did not score worse** than the
  others, which is itself worth noting rather than ignoring: several of its
  judged-relevant documents happened to share incidental vocabulary with the
  query (e.g. "turning", "makeup") even without the literal word "oxidize."
  This doesn't mean vocabulary mismatch isn't a real BM25 weakness — it means
  this particular query wasn't a strong enough test of it. A better stress
  test (and the fuller comparison against semantic/hybrid retrieval) belongs
  in the next evaluation round.

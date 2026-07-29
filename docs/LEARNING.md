# Learning Log

Working notes on the decisions behind Tried & Told, written as I make them.

## 1. Document representation, inverted index, tokenization, BuildIndex

**Document representation** — split into two structs:
- `IndexDoc`: searchable text (what the tokenizer reads)
- `DocMeta`: metadata (what search results render)

**Inverted index** — `map[string][]Posting`, where `Posting{DocID, Freq}` records
how many times a term appears in a doc. Chose a slice over a nested map for
cache-friendly sequential iteration (BM25's actual access pattern), lower memory
overhead, deterministic order, and to enable sorted merge-joins for AND/phrase
queries later.

**Tokenization** — split text using Go's `unicode` package (letter-runs and
digit-runs become separate tokens), lowercase everything. Key edge-case calls:
- **No** stopword removal in Milestone 1 — prefixes/negations (`non-`, `doesn't`)
  matter a lot in this corpus, and BM25's IDF already down-weights common words
  at this corpus size, so removal isn't needed for correctness yet.
- Hyphens and contractions (`non-comedogenic`, `doesn't`) split into pieces
  (`non`, `comedogenic`; `doesn`, `t`) rather than stripped, so negation words
  stay searchable.
- `%` and other punctuation dropped as boundaries, not kept in tokens.
- Empty/whitespace-only/punctuation-only strings produce no tokens.

**BuildIndex** — for each document, processed in strictly ascending ID order,
tokenize its text, count term frequencies in a temporary per-document map, then
append one `Posting` per term to the index. Appending (not inserting) keeps
every term's postings list already sorted by DocID without ever explicitly
sorting. The temporary map avoids mutating the index while counting, so if the
ordering assumption were ever violated, the result would be detectably
"unsorted" rather than silently wrong frequencies. The actual sortedness
guarantee is enforced separately: `BuildIndex` panics if it ever sees a
non-increasing DocID.

## 2. Initial corpus sourcing

Used a small hand-authored **synthetic** dataset (`data/synthetic/experiences.jsonl`,
one JSON object per line, ~30 entries) instead of real scraped/licensed content,
to avoid ethical issues around using sources not meant to be publicly available
or redistributable before I've actually done that licensing due diligence.

Two reasons this made sense *now* rather than blocking on real sourcing:
- This checkpoint's goal is proving the pipeline works end-to-end, not
  retrieval quality — that comes at the 5,000–10,000 doc checkpoint.
- Verifying a real dataset's license (or getting opt-in Stackd/Thread data) is
  genuine due-diligence work that shouldn't gate finishing the vertical slice.

Every entry is tagged `"source": "synthetic"` so it can never be mistaken for
real user data, and **this set does not count toward the real 500–1,000
document corpus target** — it's a disposable test fixture, not early corpus
growth. Real sourcing is separate future work.

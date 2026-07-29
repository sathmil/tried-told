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

## 3. BM25 scoring

Implemented Okapi BM25 from scratch (`internal/bm25`):
- **IDF** uses the smoothed `+1` variant, so it's always >= 0. Pairs directly
  with the no-stopword-removal decision above — since we don't filter common
  words out, IDF has to self-suppress them toward zero instead. An unsmoothed
  IDF would go negative for very common terms, actively penalizing a document
  just for containing them.
- **TF saturates at `k1+1`** as frequency grows — proven with a test that
  drives frequency to 1 billion and checks the result converges, rather than
  just asserting it.
- **`k1`/`b` are parameters**, not constants (`Params{K1, B}`), with literature
  defaults (`1.5`, `0.75`) in `DefaultParams` — tunable later via the eval
  harness, never the held-out test set.
- **`index.Index` had to grow** from a bare postings map into a struct also
  holding `DocLen []int`, `AvgDocLen`, `N` — document length is a property of
  the doc, not any term, so it's stored once per DocID rather than duplicated
  into every posting. This forced tightening `BuildIndex`'s invariant from
  "strictly ascending IDs" to "contiguous IDs starting at 0," since `DocLen`
  uses DocID directly as a slice index — the same assumption `IndexDoc`/
  `DocMeta` already made back in decision 1, just not enforced yet.
- **`Search`** accumulates each query term's contribution into
  `scores[DocID]` — the concrete payoff of giving `Posting` a `DocID` back in
  decision 1: contributions from different query terms have to land on the
  same document to sum into one total score.

## 4. Search endpoint

- Standard library `net/http` only — no router framework; not justified at
  this scale.
- The index is built once at startup and never mutated while serving, so
  concurrent request goroutines only ever *read* it. No mutex needed — Go
  maps are unsafe for concurrent read+write, but concurrent-reads-only is
  fine.
- Response is minimal on purpose: `doc_id`, `score`, `text`, `source`,
  `product`, `skin_tone`, `climate`. No snippets/highlighting/"why this
  matched" yet — that's later serving-milestone work.

## 5. Minimal UI

- One static HTML page, vanilla JS, embedded into the Go binary via
  `go:embed` — a single deployable binary with no separate frontend build
  step, which matters for the "cheap to host" goal later.
- Caught a real bug while testing in-browser: the page had no explicit
  `background-color`, so it inherited a dark shell and the text was nearly
  unreadable. Fixed with an explicit background + a `color-scheme` meta tag.
- Result text/metadata render via `textContent`-based escaping, not raw
  string interpolation into `innerHTML` — matters once real (untrusted)
  scraped content replaces the synthetic set.

## 6. Initial evaluation queries

- 10 hand-judged queries stored as **sparse qrels** — only relevant docs
  listed per query, everything else implicitly irrelevant. Exhaustive
  per-doc judging doesn't scale past a toy corpus, so the workflow has to
  already be the one that scales (same idea TREC-style qrels use).
- Computed Precision@10 and MRR now; deferred Recall@10 (meaningless without
  exhaustive judgments) and nDCG@10 (more machinery than this checkpoint
  needs) to the fuller evaluation harness at the next corpus checkpoint.
- **Result: MRR = 1.0**, but mean P@10 = 0.30. Traced the low P@10 to a real
  metric limitation, not a ranking failure: several queries have fewer than
  10 total relevant docs in the whole 30-doc corpus, which caps P@10 well
  below 1.0 even for perfect ranking. Documented in
  `docs/eval/milestone1-results.md` rather than left unexplained.
- Planted a deliberate "difficult paraphrase" query (describes oxidation
  without the word "oxidize") to stress-test vocabulary mismatch. It didn't
  actually score worse — a useful negative result, not a wasted one: the
  query wasn't a strong enough test, and a real version of this comparison
  belongs with semantic/hybrid retrieval later.

## Milestone 1 reflection

What recurred across every decision, in one line each:
- **Structure follows access pattern.** Slice vs. map, dense vs. sparse
  judgments, sorted-by-construction vs. explicit-sort — every one of these
  came down to "what does the code that reads this actually do with it,"
  not "which structure sounds more sophisticated."
- **Prove properties, don't assert them.** The BM25 saturation test doesn't
  just check a number — it drives frequency to 1e9 and checks convergence to
  `k1+1`. The IDF tests check the actual guarantee (never negative), not just
  a sample value.
- **Design for the scale after this one, not the scale after that.**
  Sparse qrels over exhaustive judging, `DocID`-indexed `DocLen` over
  per-posting length, contiguous ID enforcement — all decided by asking
  "does this still work at 50,000 docs," not by what's easiest at 30.
- **Every claim has a receipt.** No stopword removal, no dense judging, no
  quality claim — each came with a doc explaining why, and a test or a
  results table proving it, not just a comment.

What's still explicitly *not* validated: real corpus data (everything here is
synthetic), a disk-backed index (everything is in-memory), phrase/positional
search (postings don't carry positions yet), semantic retrieval (lexical-only
so far), and evaluation at any real scale (10 queries over 30 docs, not 50+
over 5,000+). All of that is the deliberate scope of Milestone 1, not an
oversight — the next real decision is what to scale first.

## Corpus sourcing, take two

Tried to shortcut real-data sourcing by asking to scrape ToS-prohibited
sites "privately, with a note." Correctly refused: a note doesn't cure a
ToS violation, the project requires a live public URL eventually anyway, and
it was the exact scenario the project's own ethics section was written to
prevent. Real research findings instead: Open Beauty Facts is genuinely
open (ODbL) but is product metadata, not first-person text; the popular
McAuley Lab Amazon Reviews 2023 dataset has **no stated license anywhere**
despite being widely used in research; the popular Sephora Kaggle review
dataset was scraped in violation of Sephora's own Terms of Use regardless of
whatever license the uploader attached to it. Conclusion: no clean,
off-the-shelf first-person beauty/skincare dataset exists. Considered
switching domains entirely to dodge this — rejected, because the sourcing
problem (UGC platforms almost universally restrict scraping) is general, not
beauty-specific, and switching would sacrifice the representation-aware
framing that's the actual point of the project. Decision: keep the domain,
pursue real opt-in collection + crawler engineering against local mocks in
parallel, keep looking for genuinely permissive sources in the background.

## 7. Crawler frontier

- Per-host FIFO queues, scheduled by a min-heap of hosts keyed on
  next-allowed-fetch time. Chosen over a single global FIFO (nothing stops
  hammering one host) and fixed-delay round-robin (wastes time waiting on
  the slowest host's rhythm even when a faster host has been ready the
  whole time) — the heap lets every host advance at its own pace and only
  blocks the worker when literally every host is in cooldown.
- `Clock` (`func() time.Time`) is injected rather than calling `time.Now()`
  directly, so tests advance time instantly instead of sleeping in real
  time to verify politeness delays.
- **Real bug caught before shipping:** the first version only remembered a
  host's cooldown while it had a live heap entry. Once a host's queue fully
  drained, that memory vanished — so new work arriving shortly after would
  be treated as immediately eligible, silently violating politeness. Fixed
  by tracking `lastFetch[host]` independently of queue/heap membership.
  Caught by a test written specifically to exercise that sequence, not by
  inspection.

## 8. URL normalization

- Canonicalizes scheme/host case, default ports, fragments, `.`/`..` path
  segments, and query parameter order. Deliberately does **not** merge
  trailing-slash variants, `www` vs. non-`www` hosts, or strip tracking
  params — all heuristic risks, not safe rewrites. Any true duplicates that
  slip through are meant to be caught later by near-duplicate content
  detection, not guessed at the URL-string level.
- **Two real bugs, both caught by testing an assumption instead of trusting
  it:**
  1. Assumed Go's `net/url` would auto-canonicalize percent-encoding on
     reserialization. A test proved it doesn't — `net/url` preserves the
     *original* escaping (`RawPath`) verbatim for round-trip fidelity.
  2. The obvious fix (clear `RawPath`) was itself wrong, and worse: since
     `net/url` fully decodes reserved characters into `Path` too, clearing
     `RawPath` caused an encoded slash (`%2F`) to silently become a real `/`
     separator — corrupting the URL's actual structure, not just its
     spelling. Fixed by decoding only *unreserved* characters (which can
     never be structurally meaningful) and running dot-segment resolution
     on the still-escaped string, so a kept `%2F` is never mistaken for a
     separator.
- The lesson that mattered more than either bug: both came from reasoning
  about what a standard library "should" do instead of writing a test that
  would fail if that reasoning was wrong. The fix each time was a test
  first, not "being more careful."

## 9. URL dedup registry + Bloom filter

- Two layers: a bit-packed Bloom filter as a fast first-pass check (never
  false-negatives, can false-positive), backed by an exact SHA-256-keyed map
  as the authoritative source of truth consulted only when the Bloom filter
  says "maybe."
- **Why the Bloom filter can't be the sole registry:** a false positive
  means "probably seen" for a URL that was never actually crawled. Trusting
  that alone silently and permanently loses real content, with nothing to
  notice later — worse than the alternative failure (redundant recrawl,
  which is just wasteful and self-correcting).
- **Sizing has a concrete payoff, not just a formula.** For 1,000,000
  expected URLs at a 1% false-positive rate: `m ≈ 9.6M bits ≈ 1.2 MB`,
  `k ≈ 7`. That ~1.2 MB structure resolves most checks instantly, versus
  16MB+ (hashes) or 100MB+ (full strings) to consult the exact registry on
  every single one.
- **Storing hashes instead of full URLs reintroduces the same risk one
  layer down.** The tradeoff isn't "URLs aren't stored" — it's hash
  collision, which is the identical false-positive failure mode as the
  Bloom filter, just at the layer that's supposed to be authoritative.
  Accepted because a 128-bit hash's collision probability at this project's
  realistic scale (thousands to low millions of URLs) is astronomically
  small.
- Used `[]uint64` with manual bit shifting for the Bloom filter's storage,
  not `[]bool` — a Go `[]bool` takes 1 byte per element, 8x the memory of a
  real bit array, which would have undercut the entire memory argument for
  using a Bloom filter at all.
- Wrote a test that deliberately forces the Bloom filter into guaranteed
  false-positive mode (`m=1` bit) and confirms the exact registry still
  gives the correct answer — a direct test of the actual reason this design
  has two layers, not just an add-then-check smoke test.

## 10. robots.txt compliance

- First external dependency in the project: `github.com/jimsmart/grobotstxt`,
  chosen over hand-rolling a parser. Unlike BM25/the inverted index (explicit
  learning targets), robots.txt parsing is protocol compliance with lots of
  easy-to-get-wrong edge cases (wildcards, Allow/Disallow precedence) — and
  getting it subtly wrong means violating a site's actual stated wishes, not
  just shipping a bug. Verified the license (Apache 2.0) and that it's a
  faithful port of Google's own reference parser (the same implementation
  RFC 9309 is based on) directly, rather than assuming — same lesson as the
  `net/url` mistakes in entry 8.
- Fetch outcome policy: 200 → real rules apply; 404 → no restrictions,
  crawl freely; anything else (error, unreachable, bad status) → **fail
  closed**, not allowed. Fail-closed follows directly from the project's
  founding principle — "don't assume permission, confirm it" — the exact
  reasoning already used to refuse scraping ToS-prohibited sites. If
  robots.txt can't be fetched, there's zero information about what's
  allowed, so proceeding would mean crawling without confirmed permission.
- **Failures are deliberately never cached.** Fail-closed doesn't have to
  mean permanently blackholing a host over one bad request — by only
  caching successful outcomes, the next `Allowed` call naturally retries the
  fetch fresh. "Not allowed yet," not "not allowed forever," with no
  separate TTL logic needed. Tested directly with a mock server that fails
  once then succeeds.

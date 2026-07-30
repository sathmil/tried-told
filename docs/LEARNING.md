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

## 11. Fetch loop

- Wired `Frontier` + `robots.Checker` + `normalize` + `dedup.Registry` into
  an actual `Fetcher.Fetch(url)` plus a `Crawl` worker pool — the first
  genuinely concurrent code in the project, verified clean under `go test
  -race`.
- **Workers sleep exactly until the frontier's next host is ready**
  (added `Frontier.NextReadyAt`), rather than busy-looping (wastes CPU) or
  polling on a fixed interval (arbitrary — either wastes time or reacts
  late). The heap already knows the exact soonest ready time, so there's no
  reason not to use it directly.
- **Redirects are intercepted manually**
  (`CheckRedirect: return http.ErrUseLastResponse`), not auto-followed —
  a redirect can land on a completely different host with different
  robots.txt rules, so auto-following would silently skip checking
  permission on the actual destination. Every hop re-runs the full
  normalize → dedup → robots.txt → fetch sequence. Proved this matters with
  a test where the redirect *target* (not the original URL) is the one
  disallowed by robots.txt, and confirmed it's still caught.
- Retries only 5xx/429/network failures, never a plain 404 — that's a
  definitive answer, not a transient one worth retrying.
- Deliberately did **not** build dynamic frontier growth (link extraction)
  here — `Crawl` runs against a fixed seed set and stops when the frontier
  drains. That's the extraction milestone's job, not this one's.
- `TestCrawl_RespectsPolitenessAcrossConcurrentWorkers` runs 4 real
  concurrent workers against two real local mock servers and checks actual
  recorded request timestamps — proving politeness holds under genuine
  concurrency, not just in the Frontier's own isolated single-threaded
  tests.

## 12. Persistent, resumable frontier + raw content storage

- **Pure write-ahead log**, not snapshots or a snapshot+WAL hybrid —
  because the project isn't at real crawl volume yet, and the alternatives
  only solve problems (log growth, recovery time) that bite at a scale not
  yet reached. Same "design for the next stage, not the one after"
  reasoning used throughout.
- Built one generic, reusable primitive (`internal/wal`: `Log[T]`/`Replay[T]`)
  rather than three bespoke logs — the frontier log, dedup log, and content
  log all share the exact same durability mechanics (append, fsync, replay,
  tolerate a torn trailing line from a crash).
- **The actual insight, not just the mechanism:** the frontier log only
  ever needs an "enqueue" event, never a "completed" one. On restart, replay
  re-adds *every* URL ever discovered, including already-fetched ones — but
  `Fetcher.Fetch`'s first step is the dedup check, so an already-fetched URL
  (if the dedup log was *also* replayed) gets skipped via `ErrAlreadySeen`
  automatically. Two simple, single-purpose logs turned out to be simpler
  than one unified log with completion-tracking and replay-ordering logic.
  Proved this end-to-end (not just per-component) with a test that fetches
  one of two URLs, "crashes," reopens both logs fresh, and confirms the
  fetched one is skipped while the new one still goes through — checked
  against actual mock-server request counts.
- Real integration wrinkle: `Fetcher` depended on the concrete
  `*dedup.Registry` type, so the persistent wrapper couldn't substitute in.
  Fixed by depending on a minimal interface instead — existing tests kept
  passing unmodified, since `*dedup.Registry` already satisfied it.
- Content storage: plain JSONL, not WARC — nothing here interoperates with
  external archive tooling yet, and everything else already uses JSONL.
  Explicitly revisit only if real WARC interop becomes a requirement.
- Deliberately left out: per-host cooldown timing doesn't survive a
  restart (resets to "ready now" — a bounded, minor politeness cost, not a
  correctness issue), and no snapshotting/compaction yet (revisit once
  replay time or disk usage is a measured problem, not before).

## Corpus sourcing, take three

Did another research pass on real sources: found nothing new with genuinely
open reuse rights. One concrete, informative result — SkincareTalk's
robots.txt now redirects to TollBit, an AI-content licensing/paywall
service, meaning that site has deliberately set up infrastructure to
*charge* crawlers rather than permit free access. Not a source to pursue,
and a sign of a broader trend (sites gating AI crawlers specifically
because uninvited scraping got so common) that only reinforces why
"confirm permission, don't assume it" matters more now, not less. Three
research rounds without a clean lead means further generic searching has
hit diminishing returns — real progress from here needs either checking
specific candidate sites one at a time, or leaning harder into opt-in
collection. Kept as a background item rather than a blocker.

## 13. Extraction architecture (site-specific extractors)

- New component: turning one fetched HTML page (a *container* - a product
  page can hold 50 reviews) into individually-searchable `Passage`s. This is
  the document-level vs. passage-level distinction: the fetched page is the
  document, but the actual unit to search/rank/return is one review, not
  the whole page.
- **Site-specific extractors over a generic boilerplate-removal algorithm**
  (e.g. a Readability port) — chosen specifically because of how sourcing
  went this session: the crawler isn't pointed at the open web generically,
  it'll end up crawling a small number of specifically-vetted sources, and
  when you already know exactly which few sites you're extracting from, a
  small per-site extractor is both simpler and far more precise than a
  generic heuristic. Explicitly not the permanent answer, though: as the
  corpus scales toward more sources (not just more pages from the same few),
  hand-writing one extractor per site stops being sustainable — the planned
  evolution is a generic-algorithm fallback for sources without a dedicated
  extractor, added once that maintenance cost becomes real, not before.
- **No real source to point this at yet**, same situation the crawler was
  in before a real target existed — built and tested against a fixture
  HTML page standing in for a hypothetical review site, ready to be
  replaced/joined by a real extractor once a real source clears vetting.
- Second external dependency: `goquery` (CSS-selector querying over Go's
  HTML parser) — verified license/maintenance status directly rather than
  assumed, same practice as `grobotstxt`. Parsing arbitrary HTML correctly
  is a real problem on its own, not a core learning objective here.
- Kept `Passage`'s metadata deliberately narrow (`Product`, `SkinTone`,
  `Climate` — same vocabulary as `index.DocMeta`) rather than building out
  the full metadata list (concern, duration, positive/negative
  observations) all at once. That's its own separate, later decision.
- Test worth naming: explicitly asserts known boilerplate fragments (nav
  text, ad copy, footer copyright) never leak into any extracted passage,
  and that a review with blank text is skipped rather than producing an
  empty passage — not just checking that the right passages came out.

## 14. Near-duplicate detection (SimHash)

- **SimHash, not MinHash** — both solve the same problem exact hashing
  can't (a single differing character produces a totally different hash),
  but they estimate different similarity notions: MinHash estimates
  Jaccard/set-overlap (fits verbatim copies), SimHash estimates
  cosine/weighted-feature similarity over a compact bit fingerprint (more
  tolerant of reordering/light rewording). My first justification for
  SimHash ("get rid of near-dups so opinions are equal") was just
  restating the shared goal, not an actual reason to prefer one over the
  other — the real reason came from thinking about what kind of
  near-duplicate this corpus will actually have: reworded/paraphrased
  reposts of the same review, not verbatim copies (those are already
  caught free by the exact-hash dedup registry). SimHash is specifically
  suited to the case exact hashing structurally cannot touch.
- Reused the existing tokenizer for features instead of building separate
  shingling — one less thing to design from scratch, and unigram tokens
  weighted by occurrence count worked fine.
- **Threshold stays conservative and explicitly provisional** — a loose
  threshold risks disproportionately erasing minority-context content
  (scarcer by definition, and more likely to look "similar to itself" by
  shared specific vocabulary even when describing genuinely different
  experiences), while majority-pattern content has plenty of redundant
  copies to spare. The exact numeric cutoff isn't locked in from theory —
  it's a parameter to `IsNearDuplicate`, meant to be validated against real
  judged examples once real data exists.
- Test worth naming: didn't assert a specific Hamming-distance number
  (a brittle magic number with no real justification) — asserted the
  *comparative* property that actually matters, that a paraphrase of a
  review lands closer than genuinely unrelated text. That's an empirical
  check the algorithm behaves as claimed on data like this project's own
  corpus, not just a theoretical assumption.

## 15. Passage segmentation

- Split at **paragraph boundaries**, not fixed-size token windows and not
  "never split a review further." Fixed windows need a size parameter to
  guess at with no real data to tune it against; paragraph breaks are a
  structural signal the author already put there, and — importantly — a
  no-op for anything that's already one paragraph, which is most of what
  this corpus looks like. Only matters for the rare long, multi-topic
  review, exactly where keeping everything in one passage would dilute
  relevance the most.
- My first instinct was "don't segment at all, I want more info" — caught
  as conflating two different things: retrieval precision (hurt by keeping
  everything together) and reading richness (doesn't actually require
  keeping everything as one indexed unit). Indexing finer for scoring
  precision costs nothing for the reader, since the full original text
  stays available to display/link back to regardless of indexing
  granularity.
- **Real bug caught before segmentation could even work:** verified
  directly (didn't assume) that goquery's `.Text()` on an element with
  multiple `<p>` children concatenates them with **no separator at all** —
  `"First.Second."`, not `"First. Second."`. Paragraph splitting is
  meaningless if the boundaries are already gone, and worse, this would
  silently merge words together across a real paragraph break once
  tokenized (`"...was high"` + `"The quality..."` → the single garbled
  token `"highthe"`). Fixed by having the extractor join `<p>` children
  with an explicit `"\n\n"` before segmentation ever sees the text.
- Test worth naming: the fixture now has a genuine multi-paragraph review,
  and the test asserts not just that it splits into two passages, but that
  no passage contains the specific merged-word failure mode the unfixed
  version would have produced — a regression test for the exact bug found,
  not just a feature test.

## 16. Language detection

- `lingua-go`, not hand-rolled — same "library for tooling, not core IR
  objectives" reasoning as `grobotstxt`/`goquery`. Matters here specifically
  because mixed-language content in one index pollutes BM25 stats (shared
  tokens with an unrelated-language passage still skew IDF), and because it
  turns "the corpus is probably mostly English" into a measurable fact
  instead of an assumption — directly useful for the representation-gaps
  documentation this project already owes.
- Picked partly because it specifically claims and tests accuracy on
  *short* text (single words/phrases), unlike most language-ID libraries
  tuned for long documents — matches our actual passages, which are short
  reviews.
- **Real gap found by testing, not assumed:** fed `"xyz"` (3 characters)
  expecting `(Unknown, false)`, got `(German, true)` instead. The
  library's own confidence flag guards against statistical ambiguity
  *between* candidate languages, not against text simply being too short
  to mean anything. Added our own minimum-length gate (20 runes) on top of
  the library's own check — same "don't invent metadata we're not
  confident about" principle used everywhere else in this component.
- Measured, not assumed: first call in a process takes ~7s (loading all 75
  language models once); every call after is fast. Worth knowing as a
  real batch-pipeline startup cost.
- **Collateral finding:** adding this package's ~35s test changed the full
  suite's parallel-execution profile enough to expose a flaky test in
  `internal/fetch` (real timing margin too tight for legitimate scheduling
  jitter under `-race` + full-suite contention — passed reliably in
  isolation every time, confirming the underlying logic was never wrong).
  Widened the test's margin rather than ignoring the flake.

## 17. Structured metadata extraction

- Two sourcing mechanisms, not one: **source-structured** fields (Product,
  ProductCategory, SkinTone, Climate — the site itself marks these up, e.g.
  `data-*` attributes) extracted via the site-specific extractor; **free-text**
  fields (duration of use, and — deferred — concern/goal, positive/negative
  observations) mentioned in review prose, requiring pattern extraction from
  the text itself.
- The "no LLM guessing" rule directly shapes free-text extraction: only
  rule-based (regex/keyword) matching is allowed, which is fully
  explainable but **deliberately low-recall by design** —
  `ExtractDuration` matches "3 weeks" but not "three weeks" or "a while."
  Not a bug — the accepted cost of refusing to guess. Better empty than
  wrong.
- Scoped duration in now (genuinely narrow pattern: digit + time unit) and
  deferred concern/sentiment extraction — not because they're less useful,
  but because there's no real data yet to validate any keyword list's
  recall against. Same "don't tune without real data" reasoning as the
  SimHash threshold and the generic-boilerplate fallback.
- Documented a seam rather than over-building: duration extraction is
  called directly inside the one extractor that exists today; noted
  explicitly that it should move to a shared post-extraction step once a
  second extractor exists, rather than building that shared step
  speculatively now.
- Test worth naming: the fixture has both a digit-based duration ("2
  weeks," matched) and a spelled-out one ("two weeks," deliberately not
  matched) in the same file — proves the positive path and the intentional
  recall boundary together, not just in isolated unit tests.

## 18. Source attribution

- A **registry**, not per-passage duplication — same reasoning as the
  extractor registry: one declared source of truth per host (type,
  license, attribution text, deletion contact) avoids drift, and makes
  policy auditable as one thing instead of scattered across every passage
  from that source.
- Closed a real gap: earlier design docs (like doc 03, for the synthetic
  corpus) documented source policy as prose for human due diligence, but
  nothing in code actually connected that documentation to what got
  served. This registry makes the policy a runtime fact, not just
  something written down once and trusted to stay true.
- **Fixed `SourceType` set, not free text** — specifically to prevent the
  harm the spec names directly: presenting a third-party review as an
  opt-in story. A closed set makes that distinction exact and queryable,
  not something a typo or inconsistent label could blur.
- `MustLookup` panics on an undeclared host rather than silently serving
  unlabeled content — content from a source with no declared policy must
  never be served, so a missing registration should fail loudly at the
  point of use. Same "panic on a should-never-happen condition" reasoning
  as `PersistentRegistry.SeenOrAdd`.
- Doesn't yet cover the Milestone 1 synthetic corpus (it uses a simpler
  flat string, no real host to key by) — noted as a deliberate scope
  boundary, revisit once real crawled and synthetic data actually need
  combining into one index.
- Test worth naming: ran the *real* extractor against the *real* fixture
  and confirmed the actual produced passages resolve through their
  `SourceURL` to the correct declared policy — not just that the registry
  behaves correctly in isolation.

## 19. Deletion & re-indexing support (extraction component complete)

- **Content-based passage identity** (`Passage.ID()`, hash of `SourceURL`
  + `Text`), not position-based — a deletion request is about removing a
  specific stated experience, not "whatever's in slot 3," and an edited
  review arguably deserves treatment as a new state to re-check rather
  than silently being assumed unchanged. Re-extracting identical content
  is idempotent (same ID every time); a real edit naturally gets a
  different ID.
- Caught and tested a real correctness detail before it could bite: joined
  `SourceURL` and `Text` with a `"\x00"` separator, not plain
  concatenation — without it, two genuinely different (URL, text) pairs
  can concatenate to the identical string and collide. Wrote a test with a
  crafted pair that would specifically expose this if the separator were
  ever removed.
- **Deletion is a WAL-backed tombstone log**
  (`crawlstate.DeletionLog`) — same append-only, single-event-type pattern
  as the frontier and dedup logs, no "undelete" event by design (matches
  "avoid unnecessary retention"). Logical delete now, physical removal
  later whenever an index gets rebuilt — the standard Lucene/LSM-tree
  pattern, not something invented for this project.
- Deferred full re-indexing (detecting genuinely changed re-crawled
  content and triggering reprocessing) — needs a real index-building
  pipeline consuming actual `Passage`s to hook into, which doesn't exist
  yet. Half the infrastructure is already there (`ContentHash` on
  `ContentRecord`); the rest waits for the consuming system to exist
  first, same reasoning as every other "defer until it's needed"
  decision this session.
- Test worth naming: extracted real passages from the real fixture,
  deleted one by its real computed ID, and confirmed exactly that one —
  and only that one — was excluded from the real remaining set. The
  extraction-to-deletion path proven together, not each piece in
  isolation.

**This closes out the extraction component**: boilerplate removal
(site-specific extractors), passage segmentation, near-dup detection
(SimHash), language detection, structured metadata, source attribution,
and now deletion/re-indexing — all built, tested, and documented across
entries 12-19.

## 20. Lexical index upgrade, part 1: sizing + delta/varint compression

- **Sizing exercise before any code**, per the project's own rule: for a
  100,000-passage target (the "deployed portfolio version" checkpoint),
  Heaps' Law (`V ≈ k·N^b`) gave a vocabulary estimate of ~6,600 terms —
  used a lower exponent (0.4) deliberately, reasoning that a narrow-domain
  corpus (skincare vocabulary repeats constantly) grows its vocabulary
  more slowly than a general-topic one would. The full estimate landed at
  ~16 MB raw for the whole postings structure — genuinely small, which
  reframes why compression matters here: it's a real technique worth
  learning and building, not a fix for a space crisis this project
  actually has at this scale.
- **Custom binary format, not SQLite/BoltDB** — sharpened an
  under-justified first answer ("not sure why") into the actual reason:
  unlike robots.txt/HTML/language-ID (peripheral tooling, correctly
  delegated to libraries), the posting-list format *is* an explicit,
  named learning objective of this project ("inverted index... built from
  scratch"). A generic embedded store would quietly delegate away exactly
  the thing this milestone exists to teach.
- **Delta + varint compression**, using `encoding/binary`'s standard
  varint rather than hand-rolling byte-packing (generic tooling) while
  building the delta *logic* ourselves (the actual technique). Applies at
  two levels with the identical mechanism: DocID gaps between postings,
  and position gaps within one posting's position list.
- **A second payoff from an old decision**: postings were sorted by DocID
  back in design doc 01, originally for merge-join AND-query performance.
  Delta encoding only works because of that same sortedness — an unsorted
  list would lose nearly all the compression benefit. The same design
  choice is now paying rent twice.
- **Real correctness guard, not just an encoding**: casting a negative
  `int` to `uint64` silently wraps to a huge garbage value rather than
  failing — the same invisible-corruption category as the earlier `%2F`
  URL-normalization bug. `EncodeDeltas` panics on non-increasing input
  rather than trusting the caller.
- **Integrity checks built in, not deferred**: `DecodeDeltas` on a
  truncated/corrupt varint returns an error, never panics or silently
  produces wrong data — the project's "integrity checks and corruption
  tests" requirement made concrete immediately, with a test that feeds it
  a deliberately truncated byte.
- Measured, not assumed: a realistic dense posting list (1,000 DocIDs,
  gap of 3) compressed to **12% of naive fixed-width size** — logged as an
  actual number in the test output, not cited from the algorithm's
  reputation.

## 21. Lexical index upgrade, part 2: immutable segment file format

- One self-contained binary file per segment (`internal/diskindex`): fixed
  header, delta+varint postings with positions, term dictionary, doc
  lengths, and an ID-mapping section, closed out with a trailing CRC32
  checksum.
- **Local sequential IDs inside postings, not `Passage.ID()` directly** —
  a 64-character hash has no numeric structure, so using it as a posting's
  DocID would have destroyed the delta-encoding compression built in entry
  20. Postings use compact local IDs (0..N-1); a separate ID-mapping
  section is the join back to the real `Passage.ID()` that deletion
  tombstones and attribution actually key on.
- **Doc lengths and the ID map are fixed-width, not delta-compressed** —
  they need random access by local ID (BM25 needs a specific doc's
  length, not a sequential scan), which is exactly the access pattern
  delta+varint is wrong for. Compression is for sequentially-scanned
  sorted data; this is the opposite pattern, so it gets the opposite
  encoding.
- Needed one real addition to `internal/postings`: `DecodeN` (decode
  exactly N values from the middle of a larger buffer, returning bytes
  consumed) — `DecodeDeltas` alone only supported "decode until the
  buffer's empty," which doesn't work when a posting's DocID gaps are
  immediately followed by more encoded data (each doc's position gaps) in
  the same block.
- **Corrected an estimate with a real measurement rather than leaving it
  looking more precise than it was.** The original ~16MB size estimate
  (entry 20) only covered postings, missing the ID-mapping section
  entirely. Building a real segment from the 30-doc synthetic corpus and
  measuring it directly surfaced the gap: 64 bytes/passage for ID mapping
  adds ~6.4MB at 100,000 passages. Same real run also confirmed the
  sizing exercise's core assumption: measured average was 19.67
  tokens/passage against an assumed 20 — close enough to trust the
  broader estimate's reasoning, not just its original arithmetic.
- Checksum verification happens **before** anything else is trusted or
  even parsed — a corrupted file is caught immediately at open time, not
  discovered later as a confusing decode failure deep in a query. A
  decode error *after* the checksum passes panics rather than returning
  an error, since that can only mean a bug in the code itself (the
  checksum already proved the bytes are exactly what was written), not a
  normal condition to handle gracefully.
- Test worth naming: `TestBuildSegment_RealExtractedPassages` runs the
  real extractor against the real fixture and round-trips the actual
  resulting passages through a real segment file — the same "prove it
  against real data, not just crafted examples" pattern used throughout
  this project.
- Deliberately not built yet: multi-segment merging, incremental indexing,
  and tombstone-aware querying (checking `DeletionLog` during search).
  This pass was "build and read one correct, immutable segment" — the
  same small-provable-units scoping as everything else this session.

## 22. Lexical index upgrade, part 3: BM25 against either backend

- `bm25.Search` now takes a small interface (`Postings`/`N`/`AvgDocLen`/
  `DocLen`) instead of the concrete in-memory `index.Index` struct, with
  adapters (`WrapInMemory`, `WrapSegment`) letting the same scoring code
  run against either backend.
- **Real reason for adapters, not methods directly on `index.Index`**:
  `index.Index` has public *fields* named `N`/`AvgDocLen`/`DocLen` — Go
  doesn't allow a method and a field to share a name on one type, so
  adding matching methods directly was never an option without renaming
  fields and breaking every existing caller. Small wrapper types sidestep
  the collision entirely without touching either backend's existing code.
- `bm25.Posting{DocID, Freq}` is deliberately backend-agnostic — the
  in-memory index has `Freq` directly; a disk segment only has positions,
  with `Freq` derived as `len(Positions)`. Each adapter does that
  translation once, at the boundary, so `Search` itself stays ignorant of
  which backend produced any given posting.
- **The test that actually matters here isn't "it returns something."**
  `TestSearch_InMemoryAndSegmentAgreeOnRealExtractedData` indexes the same
  real extracted passages both ways and asserts *identical* scores and
  rankings through both backends — a real proof the disk-backed path
  faithfully reproduces already-validated behavior, not just that it
  doesn't crash.
- Still open: passage tombstones aren't checked during search yet — a
  deleted passage still returns if its segment still contains it. Next
  natural piece, not bundled into this one.

## 23. Lexical index upgrade, part 4: phrase search

- `Segment.PhraseSearch`: fetch postings per word, intersect document sets
  (a candidate must contain every word), then check each candidate for a
  starting position where the words sit at consecutive offsets in order.
- **The real reason BM25 never needed positions, stated precisely**: it's
  a bag-of-words model — the formula only depends on *how many times* a
  term occurs, never *where*. Two documents with identical term-frequency
  profiles score identically regardless of word order. Phrase search asks
  a genuinely different question (exact adjacency and order), which is
  why it needs information BM25 structurally can't use.
- Two real optimizations, not just correctness: candidate generation
  starts from the *rarest* word (fewest postings), same principle real
  query planners use for boolean AND; the position-containment check uses
  binary search, valid specifically because positions within one posting
  are already sorted (an invariant maintained since design doc 01).
- **Honest capability gap, not papered over**: built as a method on
  `diskindex.Segment` only, not folded into the shared `bm25.Index`
  interface — the in-memory Milestone 1 index never stored positions at
  all (by design), so there's nothing for it to operate on. Rather than
  faking a no-op implementation for the backend that can't support it, the
  gap is just documented as real.
- **The test that actually matters**: a document containing "cast" and
  "white" — present, but not adjacent, and in reverse order — must NOT
  match the phrase "white cast", while a document with them genuinely
  consecutive must. Without that specific test, a buggy implementation
  that silently degraded to a plain AND query (ignoring positions
  entirely) would still pass every other test in the file.
- Deliberately not built yet: snippet generation (the other capability
  positions unlock) — wasn't asked for in this pass, so it isn't bundled
  in.

## 24. Lexical index upgrade, part 5: tombstone-aware querying

- `bm25.FilterDeleted` is a **post-processing step**, not a parameter to
  `Search` — filtering needs to resolve a result's local DocID back to a
  stable `Passage.ID()`, and that mapping only exists for segment-backed
  search. The in-memory backend has no stable passage identity at all (its
  DocIDs are ephemeral, assigned fresh per build), so baking this into
  `Search` would force every in-memory caller to pass something
  meaningless to them. Keeping scoring and deletion-filtering separate is
  what keeps `Search` genuinely backend-agnostic.
- `Deleter` is a minimal interface (`IsDeleted(passageID string) bool`),
  not a direct dependency on `crawlstate` — `*crawlstate.DeletionLog`
  already satisfies it with zero adapter code, the same "small interface,
  free satisfaction" pattern as `fetch.Deduper` and `*dedup.Registry`.
- `FilterDeleted` preserves `Search`'s ranked order — it removes entries,
  it never re-sorts the remainder.
- Test worth naming: `TestFilterDeleted_RealExtractedPassages` runs the
  entire real pipeline together — real extractor, real segment, real
  WAL-backed deletion log, real search, then deletes one of the actual
  returned results by its real `Passage.ID()` and confirms it (and only
  it) disappears. Extraction → segment → search → deletion → filtering,
  proven as one path, not five isolated pieces.

## 25. Lexical index upgrade, part 6: multi-segment querying (incremental indexing)

- **Incremental indexing itself needed no new code** — segments are
  already immutable and self-contained, so a new batch of passages is just
  another `BuildSegment` call, producing a second file, no rebuild of the
  first. The actual gap was that nothing could *query* more than one
  segment at once. `MultiSegment` closes that, not the segment format
  itself.
- Global ID translation: each segment keeps a `base` offset (first global
  ID it owns); `locate(globalID)` finds the owning segment via binary
  search over the (ascending) base offsets, same doc-base scheme Lucene
  uses for its composite reader.
- **A design simplification worth noticing, not planning for in advance**:
  `Segment` and `MultiSegment` ended up with the exact same method set, so
  one `diskindex.Queryable` interface let the existing `WrapSegment`
  handle both — no separate `WrapMultiSegment` needed. `bm25.FilterDeleted`
  got the same treatment: generalized from the concrete `*Segment` type to
  a minimal `PassageIDResolver` interface, so tombstone filtering works
  identically across one segment or many.
- Test that actually proves the point, not just "it compiles":
  `TestSearch_FindsResultsAcrossIncrementallyAddedSegments` builds two
  segments at two separate times (no rebuild of the first) and searches
  them together as one call. The deletion test specifically deletes a
  passage from the *second* segment, to prove ID translation is correct at
  a non-zero offset, not just the trivial first-segment case.
- Deliberately still deferred: segment **merging** (combining segments'
  raw postings into one, physically dropping deleted passages) — a
  genuinely harder, separate problem (needs a postings-level merge
  algorithm, not just query-time combination). `MultiSegment` makes an
  unbounded number of segments queryable; it doesn't keep that number
  bounded over time. Next piece, not this one.

## 26. Semantic indexing, part 1: offline embeddings + interchange format

First Python code in the project — scoped strictly to offline batch
generation, per the original stack mandate (Go owns indexing/serving).

- **Retrieval-tuned model over general-purpose**: `bge-small-en-v1.5`
  (MIT, 384-dim), chosen over something like `all-MiniLM-L6-v2` because
  general-purpose sentence-similarity models are trained for "are these
  two sentences paraphrases," while our actual task — find passages
  relevant to a query — is what retrieval-tuned models (contrastive,
  asymmetric objective) are specifically trained for. Verified the model's
  license/dimension/convention from its card rather than assumed.
- **Asymmetric encoding is a real correctness detail, not a footnote**:
  BGE requires an instruction prefix on queries but never on passages.
  `generate_embeddings.py` embeds passages only and deliberately adds no
  prefix — encoding a query correctly is the query service's job later,
  not this script's, and conflating the two would silently produce worse
  results.
- **The real risk in this format, named precisely**: every earlier binary
  format in this project was Go writing and Go reading — self-consistent
  even if some serialization assumption were wrong. This is the first
  format where Python writes and Go reads, a genuine cross-language
  byte-compatibility risk. Reasoned through why it should match
  (`encoding/binary.Write`'s no-padding field serialization vs. Python's
  `struct.pack("<...")` no-padding little-endian mode) — then **verified
  empirically anyway**: ran the real script, read the real output with the
  real Go reader, confirmed correct version/dimension/IDs and that vectors
  are genuinely unit-normalized (L2 norm 1.0). Reasoning predicted it;
  only the actual run proved it.
- Same header/sections/checksum format as the lexical index's segment
  file, for consistency rather than inventing a new pattern.
- `VERSION` is our own convention marker ("model + how we use it"), not
  the model's own HuggingFace revision hash — a consumer should be able to
  check this string against what it expects before trusting the file, if
  either the model or our normalization/prefix convention ever changes.
- Test worth naming: `TestOpen_RealPythonGeneratedFile` uses a fixture
  committed once from genuine script output, not regenerated at test time
  (would require Python + the actual model wherever tests run) — proof
  against real output, not just Go-authored bytes shaped to match the
  reader's own assumptions.
- Deliberately not built yet: the actual HNSW graph (build + query) from
  these vectors. This pass proves generation + interchange work
  end-to-end; wiring `github.com/coder/hnsw` is the next, separate piece.

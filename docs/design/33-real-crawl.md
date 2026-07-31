# Design Decision 33: A Small, Individually Vetted Real Crawl

Status: decided, Real-source sourcing (in progress)
Date: 2026-07-30

## Decision

`cmd/crawl` runs the real crawler (`internal/fetch`, `internal/frontier`,
`internal/robots`) against a fixed list of 5 real pages across 2 real
sites, extracts real passages via two new site-specific extractors, and
writes both a durable JSONL record (`data/real/passages.jsonl`) and a
real disk segment (`data/real/real.seg`). This deliberately does not
follow links or crawl open-endedly - `seeds` in `cmd/crawl/main.go` is
the complete, exhaustive list of URLs this crawl ever fetches.

## What "vetted" actually means here

Two candidate real sources for first-person skincare content -
`simplyemsblog.wordpress.com` and `stylexplora.blogspot.com` - were
checked directly, not assumed permitted because they're public:

- Each site's actual `robots.txt` was fetched and read in full.
  `User-agent: *` on both allows normal crawling; only admin/login/search
  paths are disallowed, and post content isn't among them.
- WordPress.com's Terms of Service were checked for any scraping
  prohibition; the only relevant clause warns against overburdening its
  infrastructure - a single-worker crawl with a 5-second per-host delay
  clearly respects that.

An earlier attempt to source real content "at scale" via existing
academic/Kaggle datasets was abandoned first: none had a clean,
independently verifiable license for this use (the Amazon Reviews 2023
academic release states no license anywhere; Yelp's dataset is explicit
but non-sublicensable, which conflicts with a public repo; a Kaggle
CC0-labeled re-upload of Amazon content couldn't be independently
verified and is a third party's claim about content they didn't
originate). Preferred de-scoping to a smaller, individually-verifiable
real crawl over taking an unclear license at face value - the same bar
this project already held when it declined to scrape a ToS-restricted
site "privately, with a note" earlier on.

## Two real gaps only a real crawl could surface

**1. The crawler never actually identified itself.** `robots.UserAgent`
existed and was already used to *match* rules within a fetched
`robots.txt`, but neither `robots.Checker` nor `fetch.Fetcher` sent that
(or any) User-Agent header on the actual HTTP requests - both used the
`client.Get` shorthand, which sends Go's generic default. Local test
fixtures never cared what header arrived; a real site's access logs
would. Fixed by building explicit `*http.Request`s with
`User-Agent: TriedAndToldBot/0.1 (+https://github.com/sathmil/tried-told)`
set, in both places, with tests proving the header actually goes out
over the wire (`TestChecker_SendsItsOwnUserAgentWhenFetchingRobotsTxt`,
`TestFetcher_SendsItsOwnUserAgent`) - the URL included so a site
operator with a question about this bot's traffic has somewhere to go.

**2. `semantic.Index.Add` only deduped against existing graph state.**
One real post repeats a lone decorative emoji as a section divider;
identical `SourceURL`+`Text` hashes to an identical `Passage.ID()`. Two
such duplicates arriving in the *same* `Add` call both passed the
existing dedup check (neither was in the graph yet) and got queued into
one underlying `hnsw.Add` call together - triggering the same
"duplicate key" library bug entry 27 already routed around, just within
a batch instead of across two calls. Fixed by deduping the incoming
batch first (last-occurrence-wins, matching the already-documented
"re-adding a passage ID replaces its vector" semantics), proven by
`TestIndex_DuplicateIDWithinOneBatchDoesNotPanic`.

The root cause of #2 was also fixed at its source, not just papered
over: `entryContentExtract` now skips any paragraph that
`tokenize.Tokenize` finds zero real terms in (`TestWordPressBlogExtractor_SkipsDecorativeNonTextParagraphs`)
- a decorative emoji is real content to a human reader, but has nothing
for BM25 or embeddings to index, and indexing it as a "passage" is what
produced the duplicate ID in the first place.

## A third gap, found but deliberately not fixed here

This same real post uses decorative Unicode "Mathematical Bold" styling
for some words (e.g. a stylized "Truffle" that is not the same Unicode
codepoints as plain ASCII "truffle"). `tokenize.Tokenize` doesn't fold
this - a plain-text search for "truffle" would never match it, even
though a human reader sees the same word. Fixing this means adding
Unicode compatibility normalization to the core tokenizer, which is
bigger than this pass and affects every indexed document, not just
crawled ones - flagged explicitly rather than fixed inline, and not
something this pass's tests should depend on either way.

## Real proof

`WordPressBlogExtractor` and `BloggerBlogExtractor` are tested against
the actual, fully unmodified HTML fetched from each real site
(`testdata/simplyemsblog_dalba_sunscreen_review.html`,
`testdata/stylexplora_skinfood_apothecary.html`) - not hand-crafted
fixtures. The crawl itself was run for real: 5 real page fetches, real
robots.txt checks, real 5-second politeness delays observed in the
timestamps, 82 real passages extracted after the junk-paragraph fix,
built into a real segment and confirmed searchable via real BM25
queries, and (offline, via the existing embedding pipeline) real
semantic search over the same real content.

## Deliberately out of scope

- Wiring `data/real/real.seg` into the live search API (`cmd/server`
  still serves the synthetic demo corpus) - a separate decision, same
  reasoning as every other "prove the mechanism, defer the wiring" split
  this session.
- Fixing the Unicode-normalization gap found above.
- Expanding beyond 5 seed URLs / 2 sites - reaching a larger real corpus
  either needs more individually-vetted sources found the same way, or
  a dataset with a genuinely clear license, neither of which this pass
  attempted to solve.

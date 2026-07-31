# Design Decision 37: Bounded Link-Following for Real-Source Scale

Status: decided, Real-source sourcing (in progress)
Date: 2026-07-31

## Decision

`cmd/crawl` now follows links, but only in a tightly bounded way: it
seeds from `listingSeeds` (category/label index pages on the two
already-vetted hosts), extracts candidate post links from each via
`discoverPostLinks`, and enqueues at most `maxDiscoveredPosts` (40) new
ones. This replaces the original fixed, hand-curated 5-URL seed list
(design doc 33) as the way this project sources real content at scale.

## Why this is safe despite being a real loosening of "fixed list only"

The original crawl's whole safety model was "the seed list is the
complete, exhaustive list of URLs ever fetched." Link-following is a
genuine loosening of that - deliberately chosen anyway, because the
actual safety guarantee was never really "nothing is discovered," it
was "nothing ungoverned gets fetched," and three separate constraints
still hold that:

1. **Discovered links are restricted to hosts already vetted** (design
   doc 33) - `discoverPostLinks` only extracts links matching
   `postURLPattern`, which is keyed by the exact same two hosts, and only
   follows links staying on the *same host* as the page they were found
   on. No new, unvetted site is ever reached this way.
2. **Every discovered URL still gets the identical per-URL robots.txt
   check** as everything else, via `fetch.Fetcher` - discovery only
   changes how a URL gets into the frontier, not what happens once it's
   there.
3. **`maxDiscoveredPosts` bounds the total** - a single run stays a
   deliberate, capped expansion, not an open-ended crawl.

## The bound caught a real mistake, immediately

Four of the seven `listingSeeds` - all four on `stylexplora.blogspot.com`
- turned out to be disallowed by that blog's own robots.txt:
`Disallow: /search` covers `/search/label/...`, which is exactly the
path Blogger's label pages live under. This wasn't noticed when that
robots.txt was first read during design doc 33's vetting - the earlier
check confirmed `Allow: /` broadly permitted post content, but didn't
specifically test a `/search/label/` path against the rule. The crawler
caught this immediately and simply skipped all four URLs with a logged
`disallowed by robots.txt` message - no manual intervention needed, and
nothing was fetched that shouldn't have been. This is the concrete case
that justifies choosing per-URL robots.txt enforcement over a purely
manually-curated list: a human (this one) got a specific URL wrong, and
the automated check caught it anyway.

Net effect: this pass's real content growth came entirely from
`simplyemsblog.wordpress.com`'s `/category/beauty/` listing (which *is*
allowed - `Disallow` there only covers admin/login/search paths, and
`/category/` isn't one of them). Growing `stylexplora.blogspot.com`
further will need a different, actually-allowed way to discover more of
its posts (e.g. its sitemap.xml, which robots.txt itself points to and
doesn't disallow) - not attempted in this pass.

## Extending, not overwriting, the existing real corpus

`cmd/crawl` now loads `data/real/passages.jsonl` (if present) at
startup, builds a set of already-crawled page URLs from it, and skips
both re-enqueueing and re-extracting anything already known - a new run
extends the corpus rather than starting over or wastefully re-fetching
pages it already has. The final segment and JSONL output are built from
the *combined* set (existing + newly crawled), in the same order the
existing corpus loader (`corpus.LoadRealCrawlJSONL`, design doc 35)
already expects, so the local-ID-to-Passage.ID() alignment that
downstream code depends on holds without any change to that loader.

## Real result

82 → 835 passages, from 45 unique real pages (5 original + 40 newly
discovered, all from one blog's `/category/beauty/` listing across its
first page - pagination to pages 2 and 3 was fetched but contributed
nothing new since the 40-post cap was already reached from page 1
alone). Passage-per-post density varied far more than the original 5
examples suggested - some "empties" round-up posts (reviewing many
products in one long post) yielded 50-70 passages each, not the
~15-20 the first crawl's small sample implied. Verified end-to-end:
`TestLoadRealCrawlJSONL_MatchesTheCommittedSegment` still passes against
the expanded corpus, embeddings were regenerated for all 835 real
passages, and the combined-corpus server was confirmed to start cleanly,
logging "serving 30 synthetic + 835 real passages."

## Deliberately out of scope

- Growing `stylexplora.blogspot.com` further (needs its sitemap.xml or
  another actually-permitted discovery path, not attempted here).
- Vetting any new platform beyond the two already checked - this pass
  widens *how much* is fetched from already-trusted sources, not *which*
  sources are trusted.
- Automatic, scheduled re-crawling - this remains a manually-run,
  deliberately bounded command, not a background job.

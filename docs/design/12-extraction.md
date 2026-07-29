# Design Decision 12: Extraction Architecture (Site-Specific Extractors)

Status: decided, Extraction component (in progress)
Date: 2026-07-29

## Decision

An `Extractor` interface (`internal/extract`) plus a `Registry` mapping
host -> `Extractor`. Each `Extractor` is **site-specific**: it knows the
exact DOM structure of the one source it was written for (e.g. "reviews
live in `.review` elements, text in a nested `.review-text`"), rather than
attempting to generically detect "boilerplate" on arbitrary pages.

## Document-level vs. passage-level retrieval

A fetched page is a **container** - a product page can hold 50 reviews, a
forum thread 40 posts. The searchable unit should be one review/passage,
not the whole page. Everything in this component exists to go from "one
fetched HTML page" to "N individually-searchable `Passage`s."

## Why site-specific over a generic algorithm, for now

Two real options: a generic boilerplate-removal algorithm (e.g. a
Readability port - works on any site via heuristics, lower precision) vs.
site-specific extractors (need to be written per source, high precision).

Chosen site-specific **because of how sourcing has gone this session**: the
crawler isn't pointed at the open web generically, it's going to end up
crawling a small number of *specifically vetted* sources. When you already
know exactly which few sites you're extracting from, a small extractor per
site is both simpler to write correctly and far more precise than a generic
heuristic algorithm trying to guess what's boilerplate on a page it's never
seen before.

**Deferred, not ignored:** this doesn't scale indefinitely. The corpus
checkpoints (500 -> 5,000 -> 50,000 -> 250,000+) likely mean *more sources*
over time, not just more pages from the same few - and hand-writing one
extractor per site stops being sustainable well before 250,000 docs. Revisit
by adding a generic-algorithm fallback (for sources without a dedicated
extractor) once the number of sources makes that maintenance cost real,
not before - same "design for the current stage" reasoning used throughout
this project.

## No real source to point this at yet

Same situation the crawler was in before a real target existed: built and
tested against a **fixture** (`testdata/example_site.html`) standing in for
a hypothetical review site, not a real one. `ExampleSiteExtractor` is a
demonstration of the pattern, ready to be replaced/joined by a real
extractor the moment a real source clears vetting.

## Library: goquery, not hand-rolled HTML parsing

Parsing arbitrary (often malformed) HTML correctly is itself a real parsing
problem, not a core IR/CS learning objective here - same reasoning as
`grobotstxt`. Verified before adding: `goquery` (BSD-3-Clause, wraps
`golang.org/x/net/html` + the `cascadia` CSS selector engine, actively
maintained, 13,800+ dependents, stable v1 API) rather than assumed from
memory.

## Passage metadata scope, deliberately narrow for now

`Passage` currently carries `Product`, `SkinTone`, `Climate` - reusing the
same vocabulary as `index.DocMeta` from Milestone 1, since passages
eventually feed into that same indexing pipeline. The fuller metadata list
in the project spec (concern/goal, duration of use, positive/negative
observations) is a separate, later decision (structured metadata
extraction is its own listed component task) - not built yet, to keep this
step to "prove the extractor pattern works," not "extract everything at
once."

## Test worth calling out

`TestExampleSiteExtractor_ExtractsPassagesAndSkipsBoilerplate` doesn't just
check that the right passages come out - it explicitly asserts that known
boilerplate fragments (nav text, ad copy, footer copyright) never appear in
any extracted passage, and that a review with blank text is skipped rather
than producing an empty passage (never invent content from nothing).

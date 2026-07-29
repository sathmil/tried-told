# Design Decision 17: Source Attribution

Status: decided, Extraction component (in progress)
Date: 2026-07-29

## Decision

A central `attribution.Registry` (keyed by host, same pattern as
`extract.Registry`) declaring each source's provenance policy once -
`Type`, `License`, `AttributionRequired`/`AttributionText`,
`DeletionContact` - rather than duplicating that policy onto every passage
that came from it.

## The gap this closes

Prior design docs (e.g. doc 03, for the synthetic corpus) document source
policy as prose a human reads during due diligence, but nothing in code
previously connected that documentation to what actually gets served.
There was no enforced guarantee that a served passage's attribution
matched what was documented. This registry makes source policy a runtime
fact, not just a doc.

## Why a registry over per-passage duplication

Same reasoning as the extractor registry: a single declared source of
truth avoids drift (correcting a source's terms means editing one entry,
not every passage from it), and centralizing the policy makes it
auditable as one thing, not scattered across however many passages exist.

## Fixed SourceType, not free text

`SourceType` is a small closed set
(`synthetic`/`opt_in`/`licensed_dataset`/`permitted_crawl`) rather than a
free-text string, specifically to prevent the exact harm the project
spec calls out: *"Do not present a third-party review as an opt-in Thread
story."* A fixed set makes that distinction an exact, queryable fact - not
something a typo or inconsistent labeling could quietly blur.

## `MustLookup`: a deliberate guardrail, not just a lookup

`Registry.Lookup` returns `(SourceInfo, bool)` for callers that can handle
absence; `Registry.MustLookup` panics if a host has no declared policy.
Content from an undeclared source must never be served - a missing
registration should fail loudly at the point of use, not silently serve
unlabeled or (worse) mislabeled content. Same "panic on a condition that
should never be silently tolerated" reasoning as
`crawlstate.PersistentRegistry.SeenOrAdd`.

## Scope boundary: doesn't yet cover the Milestone 1 synthetic corpus

The synthetic corpus (`data/synthetic/experiences.jsonl`) uses its own
simpler mechanism - a flat `Source: "synthetic"` string on `DocMeta`, with
no real URL/host to key a registry entry by. Not unified with this
registry yet, since that dataset predates the crawler pipeline and isn't
URL-addressed the way crawled content is. Revisit once real crawled
sources and the synthetic/opt-in data actually need to be combined into
one index - not before.

## Test worth calling out

`TestPassageFromExtractorResolvesToDeclaredSource` doesn't just test the
registry in isolation - it runs the real `ExampleSiteExtractor` against
the real fixture, takes the actual `Passage`s it produces, and confirms
each one's `SourceURL` resolves through `HostOf` + `MustLookup` to the
correct declared policy. Proves the wiring works end-to-end, not just that
the registry's own methods behave correctly.

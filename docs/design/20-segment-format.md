# Design Decision 20: Immutable Segment File Format

Status: decided, Lexical index upgrade (in progress)
Date: 2026-07-29

## Decision

One self-contained, immutable binary file per segment (`internal/diskindex`),
built once from a batch of `extract.Passage`s and never modified in place.

## File layout, precisely

```
[Header - fixed size, written via encoding/binary]
  Magic       [4]byte   "TTS1"
  N           uint32    number of passages in this segment
  AvgDocLen   float64   for BM25 length normalization
  PostingsOff uint64  PostingsLen uint64
  DictOff     uint64  DictLen     uint64
  DocLensOff  uint64  DocLensLen  uint64
  IDMapOff    uint64  IDMapLen    uint64

[Postings section] - per term, in dictionary order:
  DocCount        varint
  DocID gaps      delta+varint (internal/postings), DocCount values
  for each DocID:
    PosCount      varint
    Position gaps delta+varint, PosCount values
  (Freq has no field of its own - it's PosCount)

[Term dictionary] - sorted by term, for determinism:
  TermCount varint
  per term: TermLen varint, term bytes, Offset uint64 (relative to
  Postings section start), Length uint64, DF varint

[Doc lengths] - fixed-width uint32 per local DocID, in order (NOT varint -
  this section needs O(1) random access by local ID, which fixed width
  gives and variable-width delta encoding cannot)

[ID mapping] - fixed-width 64-byte extract.Passage.ID() per local DocID,
  in order (same random-access reasoning; Passage.ID() is always exactly
  64 hex characters, so this is naturally fixed-width already)

[Checksum] - trailing 4 bytes: CRC32 of everything above it
```

## Why local sequential IDs, not `Passage.ID()`, inside postings

`Passage.ID()` is a 64-character hash with no numeric structure - using it
directly as a posting's DocID would make delta encoding (design doc 19)
produce huge, incompressible deltas, destroying the compression it exists
to provide. Postings use compact sequential local IDs (0..N-1); the
ID-mapping section is the join back to the real `Passage.ID()`, which is
what deletion tombstones (`crawlstate.DeletionLog`) and the attribution
registry actually key on.

## Why doc lengths and the ID map are fixed-width, not delta-compressed

Both need **random access by local ID** - BM25 needs the length of
whichever specific doc a posting references, not a sequential scan.
Variable-width (varint) encoding is exactly the wrong tool here: entry `i`
must live at a computable offset (`i*4` or `i*64`), which only fixed-width
records give. Delta+varint is for sequentially-scanned, naturally sorted
data (postings); this is the opposite access pattern.

## Whole file loaded into memory at open time, not seek-based reads

At this project's scale (~16-23MB for 100,000 passages, corrected below),
loading the entire segment into memory is simpler than seek-based I/O and
still well within budget. Revisit (mmap or true on-demand reads) only if
corpus scale grows enough to make that untrue - not before.

## Sizing estimate, corrected against a real measurement

The original estimate (docs/design/19) covered postings only (~16MB at
100,000 passages) and didn't account for the ID-mapping section. Building
a real segment from the 30-doc synthetic corpus and measuring it directly
revealed the gap: 64 bytes/passage for ID mapping adds ~6.4MB at 100,000
passages on top of the original estimate - bringing the corrected total to
roughly 22-23MB. Still cheap, but the honest correction matters more than
the exact number: an estimate should get revised once a real measurement
exists, not left looking more precise than it was. The same real-data run
also confirmed the sizing exercise's own token-per-passage assumption:
measured average was 19.67 tokens/passage against an assumed 20.

## Integrity: checksum covers the whole file, verified before anything is trusted

`OpenSegment` verifies the CRC32 checksum first, before reading the header
or any section - a single flipped byte anywhere in the file is caught
immediately, not discovered later as a confusing downstream decode
failure. `TestOpenSegment_CorruptionIsDetected` proves this with an actual
corrupted file, not a mocked failure path. A decode error *after* the
checksum has already passed panics rather than returning an error, since
that can only mean a bug in the encode/decode logic itself, not a normal
data-quality condition - the checksum already proved the bytes are exactly
what was written.

## Deliberately out of scope for this pass

Multi-segment merging, incremental indexing (adding passages without a
full rebuild), and tombstone-aware querying (checking `DeletionLog` during
search) are not built yet - this pass is "build and read one correct,
immutable segment," consistent with how every other piece of this project
has been scoped into small, provable units rather than attempted all at
once.

## Test worth calling out

`TestBuildSegment_RealExtractedPassages` runs the real `ExampleSiteExtractor`
against the real fixture and builds/reads back an actual segment from real
passages - not just crafted test data. A separate scratch run against the
real 30-doc synthetic corpus (not committed, used only to sanity-check and
correct the sizing estimate above) confirmed the format works end-to-end
against real project data, not just test fixtures.

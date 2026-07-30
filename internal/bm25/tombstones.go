package bm25

import "triedandtold/internal/diskindex"

// Deleter reports whether a passage (identified by its stable
// extract.Passage.ID()) has been deleted. *crawlstate.DeletionLog
// satisfies this directly, with no adapter needed.
type Deleter interface {
	IsDeleted(passageID string) bool
}

// FilterDeleted removes any result whose passage has been deleted,
// resolving each result's segment-local DocID to its stable Passage.ID()
// via seg. This is deliberately a separate step from Search, not a
// parameter to it: resolving a local DocID back to a stable passage
// identity only makes sense for segment-backed search - the in-memory
// backend has no such mapping at all, so baking deletion-awareness into
// Search itself would force every in-memory caller to pass something
// irrelevant to them. Preserves Search's ranked order rather than
// re-sorting.
func FilterDeleted(results []Result, seg *diskindex.Segment, deleted Deleter) []Result {
	out := make([]Result, 0, len(results))
	for _, r := range results {
		if deleted.IsDeleted(seg.PassageID(r.DocID)) {
			continue
		}
		out = append(out, r)
	}
	return out
}

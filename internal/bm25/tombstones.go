package bm25

// Deleter reports whether a passage (identified by its stable
// extract.Passage.ID()) has been deleted. *crawlstate.DeletionLog
// satisfies this directly, with no adapter needed.
type Deleter interface {
	IsDeleted(passageID string) bool
}

// PassageIDResolver resolves a result's local DocID back to its stable
// Passage.ID() - a minimal interface (rather than depending on the full
// diskindex.Queryable) satisfied by both *diskindex.Segment and
// *diskindex.MultiSegment, since that's all FilterDeleted actually needs.
type PassageIDResolver interface {
	PassageID(docID int) string
}

// FilterDeleted removes any result whose passage has been deleted,
// resolving each result's local DocID to its stable Passage.ID() via
// resolver. This is deliberately a separate step from Search, not a
// parameter to it: resolving a local DocID back to a stable passage
// identity only makes sense for segment-backed search - the in-memory
// backend has no such mapping at all, so baking deletion-awareness into
// Search itself would force every in-memory caller to pass something
// irrelevant to them. Preserves Search's ranked order rather than
// re-sorting.
func FilterDeleted(results []Result, resolver PassageIDResolver, deleted Deleter) []Result {
	out := make([]Result, 0, len(results))
	for _, r := range results {
		if deleted.IsDeleted(resolver.PassageID(r.DocID)) {
			continue
		}
		out = append(out, r)
	}
	return out
}

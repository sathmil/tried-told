package crawlstate

import (
	"fmt"

	"triedandtold/internal/dedup"
	"triedandtold/internal/wal"
)

// SeenEntry is the dedup log's only event type: a URL was confirmed
// fetched. Replaying this log is what makes re-enqueued-but-already-done
// URLs (from EnqueueEntry replay) get correctly skipped after a restart.
type SeenEntry struct {
	URL string `json:"url"`
}

// PersistentRegistry wraps dedup.Registry with a WAL of seen events, and
// satisfies fetch.Deduper so it can be used anywhere a plain dedup.Registry
// is used.
type PersistentRegistry struct {
	registry *dedup.Registry
	log      *wal.Log[SeenEntry]
}

// OpenRegistry replays logPath (if it exists) to rebuild prior "seen"
// state, then opens it for further appends.
func OpenRegistry(logPath string, n int, p float64) (*PersistentRegistry, error) {
	r := dedup.New(n, p)

	if err := wal.Replay(logPath, func(e SeenEntry) {
		r.SeenOrAdd(e.URL)
	}); err != nil {
		return nil, err
	}

	log, err := wal.Open[SeenEntry](logPath)
	if err != nil {
		return nil, err
	}

	return &PersistentRegistry{registry: r, log: log}, nil
}

// SeenOrAdd behaves like dedup.Registry.SeenOrAdd, but durably records new
// "seen" entries so they survive a restart.
//
// The in-memory check-and-insert happens first, since dedup.Registry's
// atomicity guarantee requires it - there's no way to "peek" the answer
// without also committing it, without reintroducing the exact race that
// atomicity exists to prevent. A WAL append failure immediately afterward
// panics rather than returning an error: a failing disk write is already a
// serious operational problem that should halt the crawl regardless, and
// panicking keeps that failure loud instead of threading an error return
// through every caller for a condition that isn't meant to be recovered
// from gracefully.
func (pr *PersistentRegistry) SeenOrAdd(normalizedURL string) bool {
	if pr.registry.SeenOrAdd(normalizedURL) {
		return true
	}
	if err := pr.log.Append(SeenEntry{URL: normalizedURL}); err != nil {
		panic(fmt.Sprintf("crawlstate: failed to durably record seen URL %q: %v", normalizedURL, err))
	}
	return false
}

// Close closes the underlying log file.
func (pr *PersistentRegistry) Close() error {
	return pr.log.Close()
}

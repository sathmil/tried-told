// Package crawlstate makes the frontier and dedup registry crash-safe and
// resumable, by backing each with its own single-purpose WAL. Per
// docs/design/11-persistent-frontier.md.
package crawlstate

import (
	"time"

	"triedandtold/internal/frontier"
	"triedandtold/internal/wal"
)

// EnqueueEntry is the frontier log's only event type: a URL was enqueued.
// There is deliberately no "dequeued"/"completed" event - the dedup log
// (registry.go) already tells the fetch loop what's done, so the frontier
// log doesn't need to track completion at all.
type EnqueueEntry struct {
	Host string `json:"host"`
	URL  string `json:"url"`
}

// PersistentFrontier wraps frontier.Frontier with a WAL of enqueue events,
// replayed to reconstruct pending work after a restart.
type PersistentFrontier struct {
	frontier *frontier.Frontier
	log      *wal.Log[EnqueueEntry]
}

// OpenFrontier replays logPath (if it exists) to rebuild any previously
// enqueued work, then opens it for further appends.
func OpenFrontier(logPath string, delay time.Duration, clock frontier.Clock) (*PersistentFrontier, error) {
	f := frontier.New(delay, clock)

	if err := wal.Replay(logPath, func(e EnqueueEntry) {
		f.Add(e.Host, e.URL)
	}); err != nil {
		return nil, err
	}

	log, err := wal.Open[EnqueueEntry](logPath)
	if err != nil {
		return nil, err
	}

	return &PersistentFrontier{frontier: f, log: log}, nil
}

// Add durably records url as enqueued before adding it to the in-memory
// frontier - if the durable write fails, the in-memory Add never happens,
// so memory and disk can't silently disagree about what was recorded.
func (pf *PersistentFrontier) Add(host, url string) error {
	if err := pf.log.Append(EnqueueEntry{Host: host, URL: url}); err != nil {
		return err
	}
	pf.frontier.Add(host, url)
	return nil
}

// Next delegates to the in-memory frontier - see frontier.Frontier.Next.
func (pf *PersistentFrontier) Next() (string, bool) {
	return pf.frontier.Next()
}

// NextReadyAt delegates to the in-memory frontier - see
// frontier.Frontier.NextReadyAt.
func (pf *PersistentFrontier) NextReadyAt() (time.Time, bool) {
	return pf.frontier.NextReadyAt()
}

// Close closes the underlying log file.
func (pf *PersistentFrontier) Close() error {
	return pf.log.Close()
}

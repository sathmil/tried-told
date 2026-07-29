package crawlstate

import (
	"sync"
	"time"

	"triedandtold/internal/wal"
)

// DeletionEntry is the deletion log's only event type: a passage was
// deleted. There is deliberately no "undelete" event - deletions are
// permanent by design, matching the "avoid unnecessary retention"
// principle this exists to serve.
type DeletionEntry struct {
	PassageID   string    `json:"passage_id"`
	Reason      string    `json:"reason"`
	RequestedAt time.Time `json:"requested_at"`
}

// DeletionLog tracks which passage IDs (extract.Passage.ID) have been
// durably deleted, so a deletion request survives a restart and a future
// index-build can exclude deleted passages.
type DeletionLog struct {
	mu      sync.Mutex
	deleted map[string]bool
	log     *wal.Log[DeletionEntry]
}

// OpenDeletionLog replays logPath (if it exists) to rebuild the set of
// previously deleted passage IDs, then opens it for further appends.
func OpenDeletionLog(logPath string) (*DeletionLog, error) {
	deleted := make(map[string]bool)
	if err := wal.Replay(logPath, func(e DeletionEntry) {
		deleted[e.PassageID] = true
	}); err != nil {
		return nil, err
	}

	log, err := wal.Open[DeletionEntry](logPath)
	if err != nil {
		return nil, err
	}

	return &DeletionLog{deleted: deleted, log: log}, nil
}

// Delete durably records passageID as deleted.
func (d *DeletionLog) Delete(passageID, reason string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if err := d.log.Append(DeletionEntry{
		PassageID:   passageID,
		Reason:      reason,
		RequestedAt: time.Now(),
	}); err != nil {
		return err
	}
	d.deleted[passageID] = true
	return nil
}

// IsDeleted reports whether passageID has been deleted.
func (d *DeletionLog) IsDeleted(passageID string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.deleted[passageID]
}

// Close closes the underlying log file.
func (d *DeletionLog) Close() error {
	return d.log.Close()
}

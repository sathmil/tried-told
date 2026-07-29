// Package wal implements a generic append-only, JSONL write-ahead log:
// append an event, fsync it durable, and replay every past event on
// startup to reconstruct in-memory state after a crash or restart. Per
// docs/design/11-persistent-frontier.md.
package wal

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"sync"
)

// Log is an append-only JSONL log of entries of type T.
type Log[T any] struct {
	mu   sync.Mutex
	file *os.File
}

// Open opens (creating if necessary) the log at path for appending.
func Open[T any](path string) (*Log[T], error) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}
	return &Log[T]{file: f}, nil
}

// Append durably records entry: written and fsynced before returning, so a
// crash immediately afterward cannot lose it.
func (l *Log[T]) Append(entry T) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	data = append(data, '\n')

	if _, err := l.file.Write(data); err != nil {
		return err
	}
	return l.file.Sync()
}

// Close closes the underlying file.
func (l *Log[T]) Close() error {
	return l.file.Close()
}

// Replay reads every entry previously appended to the log at path, calling
// fn for each one in order. If the file doesn't exist yet, that's treated
// as an empty log (nothing to replay), not an error.
//
// A crash mid-write can only ever leave the *last* line truncated or
// malformed - everything before it was already fsynced by a prior Append.
// Replay stops at the first line it can't parse rather than erroring the
// whole replay, since that's expected recovery behavior, not corruption:
// everything replayed before that point is still valid and intact.
func Replay[T any](path string, fn func(T)) error {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024) // allow large lines (e.g. page bodies)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var entry T
		if err := json.Unmarshal(line, &entry); err != nil {
			break // torn trailing write from a crash - stop, don't error
		}
		fn(entry)
	}
	return scanner.Err()
}

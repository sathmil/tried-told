// Package dedup implements URL deduplication: a Bloom filter as a fast
// first-pass filter, backed by an exact registry as the authoritative
// source of truth, per docs/design/08-url-dedup.md.
package dedup

import (
	"crypto/sha256"
	"sync"
)

// urlKey is a truncated SHA-256 digest (128 bits) of a normalized URL,
// trading a negligible collision probability at this project's realistic
// scale for far less memory than storing full URL strings.
type urlKey [16]byte

func hashKey(normalizedURL string) urlKey {
	sum := sha256.Sum256([]byte(normalizedURL))
	var key urlKey
	copy(key[:], sum[:16])
	return key
}

// Registry deduplicates normalized URLs. Callers are responsible for
// normalizing (see internal/normalize) before calling - Registry compares
// exactly, it does not know how to canonicalize.
type Registry struct {
	mu    sync.Mutex
	bloom *bloomFilter
	exact map[urlKey]struct{}
}

// New creates a Registry sized for n expected URLs at Bloom false-positive
// rate p.
func New(n int, p float64) *Registry {
	return &Registry{
		bloom: newBloomFilter(n, p),
		exact: make(map[urlKey]struct{}),
	}
}

// SeenOrAdd reports whether normalizedURL has been seen before. If not, it
// records the URL as seen (in both layers) before returning false, so a
// single call is both the check and the insert.
func (r *Registry) SeenOrAdd(normalizedURL string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := hashKey(normalizedURL)

	if !r.bloom.MightContain(normalizedURL) {
		// Bloom filter is authoritative here: this can only be a true
		// negative, never a false one.
		r.bloom.Add(normalizedURL)
		r.exact[key] = struct{}{}
		return false
	}

	// Bloom filter said "maybe" - it is not trustworthy on its own, so
	// the exact registry is the one that actually decides.
	if _, ok := r.exact[key]; ok {
		return true
	}
	r.bloom.Add(normalizedURL)
	r.exact[key] = struct{}{}
	return false
}

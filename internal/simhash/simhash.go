// Package simhash detects reworded/paraphrased near-duplicate passages -
// the case exact hashing structurally cannot catch, since it looks for
// approximate similarity rather than byte-for-byte equality. Per
// docs/design/13-simhash.md.
package simhash

import (
	"hash/fnv"
	"math/bits"

	"triedandtold/internal/tokenize"
)

// Fingerprint computes a 64-bit SimHash fingerprint for text: each token
// (via tokenize.Tokenize) is a weighted feature, contributing once per
// occurrence, so more frequent tokens have proportionally more influence.
// Similar text - even reordered or lightly reworded - produces fingerprints
// with a small Hamming distance; dissimilar text produces large distances.
func Fingerprint(text string) uint64 {
	var weights [64]int
	for _, tok := range tokenize.Tokenize(text) {
		h := hashToken(tok)
		for bit := 0; bit < 64; bit++ {
			if h&(1<<bit) != 0 {
				weights[bit]++
			} else {
				weights[bit]--
			}
		}
	}

	var fp uint64
	for bit := 0; bit < 64; bit++ {
		if weights[bit] > 0 {
			fp |= 1 << bit
		}
	}
	return fp
}

func hashToken(token string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(token))
	return h.Sum64()
}

// HammingDistance returns the number of bit positions at which a and b
// differ.
func HammingDistance(a, b uint64) int {
	return bits.OnesCount64(a ^ b)
}

// IsNearDuplicate reports whether a and b are close enough (Hamming
// distance <= maxDistance) to be treated as near-duplicates. maxDistance
// should be validated against real judged examples once real data exists -
// see docs/design/13-simhash.md.
func IsNearDuplicate(a, b uint64, maxDistance int) bool {
	return HammingDistance(a, b) <= maxDistance
}

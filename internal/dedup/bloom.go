package dedup

import (
	"hash/fnv"
	"math"
)

// bloomFilter is a bit-packed probabilistic set: MightContain never false-
// negatives (if it says "no", the item was definitely never added), but can
// false-positive (it may say "maybe" for an item that was never added).
type bloomFilter struct {
	bits []uint64 // true bit-packing: m bits in m/64 words, not m bytes
	m    uint64   // number of bits
	k    int      // number of hash functions
}

// newBloomFilter sizes itself for n expected items at target false-positive
// rate p, using the standard formulas m = -(n*ln p)/(ln 2)^2, k = (m/n)*ln 2.
func newBloomFilter(n int, p float64) *bloomFilter {
	m := optimalM(n, p)
	k := optimalK(m, n)
	words := (m + 63) / 64
	return &bloomFilter{bits: make([]uint64, words), m: m, k: k}
}

func optimalM(n int, p float64) uint64 {
	m := -(float64(n) * math.Log(p)) / (math.Ln2 * math.Ln2)
	return uint64(math.Ceil(m))
}

func optimalK(m uint64, n int) int {
	k := int(math.Round((float64(m) / float64(n)) * math.Ln2))
	if k < 1 {
		k = 1
	}
	return k
}

// Add sets this item's k bit positions.
func (b *bloomFilter) Add(item string) {
	for _, pos := range b.positions(item) {
		b.bits[pos/64] |= 1 << (pos % 64)
	}
}

// MightContain reports whether item was possibly added before. False means
// definitely not; true means probably, subject to the filter's false-
// positive rate.
func (b *bloomFilter) MightContain(item string) bool {
	for _, pos := range b.positions(item) {
		if b.bits[pos/64]&(1<<(pos%64)) == 0 {
			return false
		}
	}
	return true
}

// positions simulates k independent hash functions from two real ones
// (Kirsch-Mitzenmacher double hashing: g_i(x) = h1(x) + i*h2(x)), avoiding
// the need for k genuinely independent hash functions.
func (b *bloomFilter) positions(item string) []uint64 {
	h1, h2 := doubleHash(item)
	positions := make([]uint64, b.k)
	for i := 0; i < b.k; i++ {
		positions[i] = (h1 + uint64(i)*h2) % b.m
	}
	return positions
}

func doubleHash(s string) (uint64, uint64) {
	h1 := fnv.New64a()
	h1.Write([]byte(s))
	sum1 := h1.Sum64()

	h2 := fnv.New64a()
	h2.Write([]byte{0xFF}) // seed prefix so sum2 decorrelates from sum1
	h2.Write([]byte(s))
	sum2 := h2.Sum64()

	return sum1, sum2
}

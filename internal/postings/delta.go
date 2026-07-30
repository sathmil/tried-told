// Package postings implements posting-list compression: delta encoding
// (storing gaps between consecutive sorted values, exploiting sortedness)
// combined with variable-byte encoding (encoding/binary's varint,
// exploiting that most gaps are small), per
// docs/design/19-delta-varint.md. Used both for DocID gaps between
// postings and for position gaps within a single posting's position list -
// the same technique, one level down.
package postings

import (
	"encoding/binary"
	"fmt"
)

// EncodeDeltas delta-encodes a strictly increasing sequence of
// non-negative integers and varint-encodes each resulting delta.
//
// Panics if the input isn't strictly increasing: a decreasing or repeated
// value would produce a negative delta, which silently wraps around to a
// huge, corrupt value when cast to uint64 rather than failing loudly - so
// this is checked explicitly rather than trusted.
func EncodeDeltas(sorted []int) []byte {
	buf := make([]byte, 0, len(sorted)*binary.MaxVarintLen64)
	scratch := make([]byte, binary.MaxVarintLen64)

	prev := 0
	for i, v := range sorted {
		if i == 0 && v < 0 {
			panic(fmt.Sprintf("postings: EncodeDeltas requires non-negative values, got %d", v))
		}
		if i > 0 && v <= prev {
			panic(fmt.Sprintf("postings: EncodeDeltas requires strictly increasing input, got %d after %d", v, prev))
		}
		delta := v - prev
		n := binary.PutUvarint(scratch, uint64(delta))
		buf = append(buf, scratch[:n]...)
		prev = v
	}
	return buf
}

// DecodeDeltas reverses EncodeDeltas, reconstructing the original strictly
// increasing sequence. Returns an error (never panics or silently produces
// garbage) if data is truncated or otherwise not valid varint-encoded
// deltas - e.g. from disk corruption.
func DecodeDeltas(data []byte) ([]int, error) {
	var out []int
	prev := 0
	for len(data) > 0 {
		delta, n := binary.Uvarint(data)
		if n <= 0 {
			return nil, fmt.Errorf("postings: corrupt or truncated varint, %d bytes remaining", len(data))
		}
		prev += int(delta)
		out = append(out, prev)
		data = data[n:]
	}
	return out, nil
}

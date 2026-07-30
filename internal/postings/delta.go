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
// increasing sequence from a buffer containing nothing else. Returns an
// error (never panics or silently produces garbage) if data is truncated
// or otherwise not valid varint-encoded deltas - e.g. from disk
// corruption.
func DecodeDeltas(data []byte) ([]int, error) {
	values, _, err := decodeDeltasN(data, -1)
	return values, err
}

// DecodeN decodes exactly count delta-encoded values from the start of
// data, returning the reconstructed values and the number of bytes
// consumed. For reading a sequence embedded inside a larger buffer
// alongside other encoded data (e.g. a posting's DocID gaps immediately
// followed by each document's position gaps) - unlike DecodeDeltas, it
// doesn't require the sequence to be the only thing in data.
func DecodeN(data []byte, count int) (values []int, bytesConsumed int, err error) {
	return decodeDeltasN(data, count)
}

// decodeDeltasN is the shared decode loop: if count < 0, decode until data
// is exhausted (DecodeDeltas' behavior); otherwise decode exactly count
// values (DecodeN's behavior).
func decodeDeltasN(data []byte, count int) ([]int, int, error) {
	var out []int
	prev := 0
	pos := 0
	for count < 0 || len(out) < count {
		if count < 0 && pos >= len(data) {
			break
		}
		delta, n := binary.Uvarint(data[pos:])
		if n <= 0 {
			return nil, 0, fmt.Errorf("postings: corrupt or truncated varint at byte %d", pos)
		}
		prev += int(delta)
		out = append(out, prev)
		pos += n
	}
	return out, pos, nil
}

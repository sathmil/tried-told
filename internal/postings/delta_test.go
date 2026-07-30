package postings

import (
	"slices"
	"testing"
)

func TestDecodeN_ReadsExactlyCountValuesFromALargerBuffer(t *testing.T) {
	first := EncodeDeltas([]int{5, 8, 9})   // 3 values
	second := EncodeDeltas([]int{100, 200}) // a second, independent sequence appended right after

	combined := append(append([]byte{}, first...), second...)

	gotFirst, consumed, err := DecodeN(combined, 3)
	if err != nil {
		t.Fatalf("DecodeN returned error: %v", err)
	}
	if !slices.Equal(gotFirst, []int{5, 8, 9}) {
		t.Errorf("DecodeN got %v, want [5 8 9]", gotFirst)
	}
	if consumed != len(first) {
		t.Errorf("consumed = %d, want %d (exactly the first sequence's byte length)", consumed, len(first))
	}

	// The whole point: decoding continues correctly from where the first
	// call left off, proving `consumed` is trustworthy for sequential
	// parsing of a larger buffer.
	gotSecond, err := DecodeDeltas(combined[consumed:])
	if err != nil {
		t.Fatalf("DecodeDeltas on the remainder returned error: %v", err)
	}
	if !slices.Equal(gotSecond, []int{100, 200}) {
		t.Errorf("second sequence = %v, want [100 200]", gotSecond)
	}
}

func TestDecodeN_TruncatedInputReturnsError(t *testing.T) {
	encoded := EncodeDeltas([]int{5, 8, 9})
	// Ask for more values than are actually present.
	if _, _, err := DecodeN(encoded, 5); err == nil {
		t.Error("DecodeN with count exceeding available data returned no error")
	}
}

func TestEncodeDecodeDeltas_RoundTrip(t *testing.T) {
	cases := [][]int{
		{7, 8, 20, 21, 22, 23}, // the hand-worked example
		{0},
		{5},
		{0, 1, 2, 3, 4},
		{1, 1_000_000}, // forces a multi-byte varint
		{},
	}

	for _, want := range cases {
		encoded := EncodeDeltas(want)
		got, err := DecodeDeltas(encoded)
		if err != nil {
			t.Fatalf("DecodeDeltas returned error for %v: %v", want, err)
		}
		if len(want) == 0 {
			if len(got) != 0 {
				t.Errorf("DecodeDeltas(EncodeDeltas(%v)) = %v, want empty", want, got)
			}
			continue
		}
		if !slices.Equal(got, want) {
			t.Errorf("DecodeDeltas(EncodeDeltas(%v)) = %v, want %v", want, got, want)
		}
	}
}

func TestEncodeDeltas_PanicsOnNonIncreasingInput(t *testing.T) {
	cases := [][]int{
		{5, 5},    // duplicate
		{5, 3},    // decreasing
		{5, 4, 6}, // decreases then recovers - still invalid
	}
	for _, input := range cases {
		func() {
			defer func() {
				if r := recover(); r == nil {
					t.Errorf("EncodeDeltas(%v) did not panic on non-increasing input", input)
				}
			}()
			EncodeDeltas(input)
		}()
	}
}

func TestEncodeDeltas_PanicsOnNegativeFirstValue(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("EncodeDeltas([-1]) did not panic")
		}
	}()
	EncodeDeltas([]int{-1})
}

// TestDecodeDeltas_CorruptionReturnsErrorNotPanic is the "integrity checks
// and corruption tests" requirement made concrete: a truncated varint
// (e.g. from disk corruption) must fail loudly with an error, never panic
// the whole process and never silently return wrong data.
func TestDecodeDeltas_CorruptionReturnsErrorNotPanic(t *testing.T) {
	// A varint byte with its continuation bit set (>= 0x80) but no
	// following byte - a truncated/corrupt encoding.
	corrupt := []byte{0x80}

	_, err := DecodeDeltas(corrupt)
	if err == nil {
		t.Error("DecodeDeltas on truncated input returned no error")
	}
}

// TestEncodeDeltas_CompressesRealisticPostingList makes the compression
// claim reproducible rather than assumed: verifies delta+varint actually
// uses fewer bytes than naive fixed-width encoding for a posting list
// shaped like what a common term's DocIDs would look like (dense, mostly
// small gaps).
func TestEncodeDeltas_CompressesRealisticPostingList(t *testing.T) {
	docIDs := make([]int, 1000)
	for i := range docIDs {
		docIDs[i] = i * 3 // gaps of 3 throughout - realistic for a fairly common term
	}

	encoded := EncodeDeltas(docIDs)
	naiveFixedWidth := len(docIDs) * 8 // a naive []int64 encoding, 8 bytes each

	if len(encoded) >= naiveFixedWidth {
		t.Errorf("delta+varint encoding used %d bytes, naive fixed-width would use %d - expected real compression",
			len(encoded), naiveFixedWidth)
	}
	t.Logf("1000 DocIDs (gap=3): delta+varint = %d bytes, naive fixed-width = %d bytes (%.0f%% of naive)",
		len(encoded), naiveFixedWidth, 100*float64(len(encoded))/float64(naiveFixedWidth))
}

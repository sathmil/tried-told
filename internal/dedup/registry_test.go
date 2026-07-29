package dedup

import "testing"

func TestRegistry_FirstSeenReturnsFalseThenTrue(t *testing.T) {
	r := New(100, 0.01)

	if r.SeenOrAdd("https://a.com/1") {
		t.Fatal("first call for a new URL returned true (already seen)")
	}
	if !r.SeenOrAdd("https://a.com/1") {
		t.Fatal("second call for the same URL returned false (not seen)")
	}
}

func TestRegistry_DifferentURLsAreIndependent(t *testing.T) {
	r := New(100, 0.01)
	r.SeenOrAdd("https://a.com/1")

	if r.SeenOrAdd("https://a.com/2") {
		t.Fatal("a different, never-seen URL was reported as already seen")
	}
}

// TestRegistry_ExactLayerOverridesBloomFalsePositive is the core property
// this whole two-layer design exists for: forces the Bloom filter into
// guaranteed false-positive mode (m=1 bit, so any Add sets the only bit,
// making MightContain true for everything afterward) and confirms the
// exact registry still gives the objectively correct answer rather than
// trusting the Bloom filter's "maybe."
func TestRegistry_ExactLayerOverridesBloomFalsePositive(t *testing.T) {
	r := New(1, 0.9) // n=1, p=0.9 -> m=1 bit, k=1: guarantees false positives

	r.SeenOrAdd("https://a.com/1")

	// The Bloom filter now says "maybe" for literally anything, including
	// a URL that was never added - the exact registry must still catch
	// this and report it correctly as new.
	if r.SeenOrAdd("https://b.com/2") {
		t.Fatal("registry reported an unseen URL as seen - the Bloom filter's false positive was trusted instead of overridden by the exact registry")
	}

	// Having just been added by the previous call, it's now genuinely seen.
	if !r.SeenOrAdd("https://b.com/2") {
		t.Fatal("URL added in the previous call was not recognized as seen")
	}
}

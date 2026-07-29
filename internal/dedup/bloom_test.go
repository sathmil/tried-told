package dedup

import "testing"

func TestOptimalMK_MatchesKnownBallpark(t *testing.T) {
	// n=1,000,000 items, p=1% false-positive rate: textbook ballpark is
	// ~9.6M bits and ~7 hash functions.
	m := optimalM(1_000_000, 0.01)
	if m < 9_500_000 || m > 9_700_000 {
		t.Errorf("optimalM(1_000_000, 0.01) = %d, want ~9,585,000", m)
	}

	k := optimalK(m, 1_000_000)
	if k != 7 {
		t.Errorf("optimalK(%d, 1_000_000) = %d, want 7", m, k)
	}
}

func TestBloomFilter_NeverFalseNegative(t *testing.T) {
	b := newBloomFilter(100, 0.01)
	added := []string{
		"https://a.com/1", "https://a.com/2", "https://b.com/1",
		"https://c.com/x?y=1", "https://d.com/",
	}
	for _, item := range added {
		b.Add(item)
	}
	for _, item := range added {
		if !b.MightContain(item) {
			t.Errorf("MightContain(%q) = false after Add - a false negative, which must never happen", item)
		}
	}
}

func TestBloomFilter_UnaddedItemReportsNotPresent(t *testing.T) {
	// Sized generously relative to what's actually inserted, so a false
	// positive on this specific probe string is vanishingly unlikely.
	b := newBloomFilter(1000, 0.01)
	b.Add("https://a.com/1")
	b.Add("https://b.com/2")

	if b.MightContain("https://never-added.example/some-other-path") {
		t.Error("MightContain reported true for an item that was never added")
	}
}

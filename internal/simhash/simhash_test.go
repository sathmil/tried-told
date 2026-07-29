package simhash

import "testing"

func TestFingerprint_IdenticalTextMatchesExactly(t *testing.T) {
	text := "This sunscreen didn't leave a white cast on my deep skin."
	if Fingerprint(text) != Fingerprint(text) {
		t.Error("identical text produced different fingerprints")
	}
}

func TestHammingDistance_IdenticalFingerprintsAreZero(t *testing.T) {
	fp := Fingerprint("some review text")
	if d := HammingDistance(fp, fp); d != 0 {
		t.Errorf("HammingDistance(fp, fp) = %d, want 0", d)
	}
}

func TestHammingDistance_KnownBitPatterns(t *testing.T) {
	cases := []struct {
		a, b uint64
		want int
	}{
		{0b0000, 0b0000, 0},
		{0b0001, 0b0000, 1},
		{0b1111, 0b0000, 4},
		{0b1010, 0b0101, 4},
	}
	for _, tc := range cases {
		if got := HammingDistance(tc.a, tc.b); got != tc.want {
			t.Errorf("HammingDistance(%b, %b) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestFingerprint_ParaphrasedTextIsCloserThanUnrelatedText(t *testing.T) {
	original := "This sunscreen didn't leave a white cast on my deep skin in humid weather."
	paraphrase := "This sunscreen left no white cast on my deep skin during humid weather."
	unrelated := "The customer service response time was excellent and the packaging arrived undamaged."

	dParaphrase := HammingDistance(Fingerprint(original), Fingerprint(paraphrase))
	dUnrelated := HammingDistance(Fingerprint(original), Fingerprint(unrelated))

	if dParaphrase >= dUnrelated {
		t.Errorf("paraphrase distance (%d) should be smaller than unrelated-text distance (%d)", dParaphrase, dUnrelated)
	}
}

func TestIsNearDuplicate_RespectsThreshold(t *testing.T) {
	a := Fingerprint("This sunscreen didn't leave a white cast on my deep skin in humid weather.")
	b := Fingerprint("This sunscreen left no white cast on my deep skin during humid weather.")
	d := HammingDistance(a, b)

	if !IsNearDuplicate(a, b, d) {
		t.Error("IsNearDuplicate should be true when maxDistance equals the actual distance")
	}
	if IsNearDuplicate(a, b, d-1) {
		t.Error("IsNearDuplicate should be false when maxDistance is stricter than the actual distance")
	}
}

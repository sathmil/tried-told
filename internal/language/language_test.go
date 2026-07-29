package language

import "testing"

func TestDetect_EnglishText(t *testing.T) {
	text := "This sunscreen didn't leave a white cast on my deep skin, even in humid weather."
	code, ok := Detect(text)
	if !ok {
		t.Fatalf("Detect(%q) returned ok=false, want a confident result", text)
	}
	if code != "en" {
		t.Errorf("Detect(%q) = %q, want %q", text, code, "en")
	}
}

func TestDetect_FrenchText(t *testing.T) {
	text := "Cette creme hydratante a beaucoup ameliore ma peau seche pendant l'hiver."
	code, ok := Detect(text)
	if !ok {
		t.Fatalf("Detect(%q) returned ok=false, want a confident result", text)
	}
	if code != "fr" {
		t.Errorf("Detect(%q) = %q, want %q", text, code, "fr")
	}
}

func TestDetect_TooShortReturnsNotOK(t *testing.T) {
	// "xyz" was verified separately to produce a confident-looking wrong
	// result from the underlying library - this is our own length gate
	// catching what the library's confidence check alone does not.
	code, ok := Detect("xyz")
	if ok {
		t.Errorf("Detect(\"xyz\") = (%q, true), want ok=false - too short to trust any result", code)
	}
}

func TestDetect_EmptyStringReturnsNotOK(t *testing.T) {
	if _, ok := Detect(""); ok {
		t.Error("Detect(\"\") returned ok=true")
	}
}

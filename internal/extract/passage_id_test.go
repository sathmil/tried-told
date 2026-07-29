package extract

import "testing"

func TestPassage_ID_SameContentProducesSameID(t *testing.T) {
	a := Passage{SourceURL: "https://example.com/1", Text: "great product"}
	b := Passage{SourceURL: "https://example.com/1", Text: "great product"}
	if a.ID() != b.ID() {
		t.Error("identical SourceURL+Text produced different IDs - re-extraction would not be idempotent")
	}
}

func TestPassage_ID_DifferentTextProducesDifferentID(t *testing.T) {
	a := Passage{SourceURL: "https://example.com/1", Text: "great product"}
	b := Passage{SourceURL: "https://example.com/1", Text: "great product, edited"}
	if a.ID() == b.ID() {
		t.Error("different Text produced the same ID")
	}
}

func TestPassage_ID_DifferentSourceProducesDifferentID(t *testing.T) {
	a := Passage{SourceURL: "https://example.com/1", Text: "great product"}
	b := Passage{SourceURL: "https://example.com/2", Text: "great product"}
	if a.ID() == b.ID() {
		t.Error("different SourceURL produced the same ID")
	}
}

func TestPassage_ID_NoConcatenationCollisionAcrossFieldBoundary(t *testing.T) {
	// Without a separator between SourceURL and Text, these two would
	// concatenate to the identical string "https://a.com/bc" and hash the
	// same, despite being genuinely different passages.
	a := Passage{SourceURL: "https://a.com/b", Text: "c"}
	b := Passage{SourceURL: "https://a.com", Text: "/bc"}
	if a.ID() == b.ID() {
		t.Error("field-boundary concatenation collision: two different (SourceURL, Text) pairs produced the same ID")
	}
}

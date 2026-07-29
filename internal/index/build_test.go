package index

import (
	"slices"
	"testing"
)

func TestBuildIndex(t *testing.T) {
	t.Run("repeated term within one document", func(t *testing.T) {
		idx := BuildIndex([]IndexDoc{{ID: 0, Text: "sunscreen sunscreen sunscreen"}})
		want := []Posting{{DocID: 0, Freq: 3}}
		if !slices.Equal(idx["sunscreen"], want) {
			t.Errorf(`idx["sunscreen"] = %v, want %v`, idx["sunscreen"], want)
		}
	})

	t.Run("term shared across documents, sorted by DocID", func(t *testing.T) {
		idx := BuildIndex([]IndexDoc{
			{ID: 0, Text: "sunscreen is great"},
			{ID: 1, Text: "sunscreen broke me out"},
		})
		want := []Posting{{DocID: 0, Freq: 1}, {DocID: 1, Freq: 1}}
		if !slices.Equal(idx["sunscreen"], want) {
			t.Errorf(`idx["sunscreen"] = %v, want %v`, idx["sunscreen"], want)
		}
	})

	t.Run("document with no tokens contributes nothing", func(t *testing.T) {
		idx := BuildIndex([]IndexDoc{{ID: 0, Text: "!!!"}})
		if len(idx) != 0 {
			t.Errorf("BuildIndex = %v, want empty index", idx)
		}
	})

	t.Run("empty docs slice produces empty index", func(t *testing.T) {
		idx := BuildIndex(nil)
		if len(idx) != 0 {
			t.Errorf("BuildIndex(nil) = %v, want empty index", idx)
		}
	})

	t.Run("out-of-order doc IDs panic", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic on out-of-order doc IDs, got none")
			}
		}()
		BuildIndex([]IndexDoc{{ID: 1, Text: "a"}, {ID: 0, Text: "b"}})
	})

	t.Run("duplicate doc IDs panic", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic on duplicate doc IDs, got none")
			}
		}()
		BuildIndex([]IndexDoc{{ID: 0, Text: "a"}, {ID: 0, Text: "b"}})
	})
}

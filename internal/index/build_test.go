package index

import (
	"slices"
	"testing"
)

func TestBuildIndex(t *testing.T) {
	t.Run("repeated term within one document", func(t *testing.T) {
		idx := BuildIndex([]IndexDoc{{ID: 0, Text: "sunscreen sunscreen sunscreen"}})
		want := []Posting{{DocID: 0, Freq: 3}}
		if !slices.Equal(idx.Postings["sunscreen"], want) {
			t.Errorf(`idx.Postings["sunscreen"] = %v, want %v`, idx.Postings["sunscreen"], want)
		}
	})

	t.Run("term shared across documents, sorted by DocID", func(t *testing.T) {
		idx := BuildIndex([]IndexDoc{
			{ID: 0, Text: "sunscreen is great"},
			{ID: 1, Text: "sunscreen broke me out"},
		})
		want := []Posting{{DocID: 0, Freq: 1}, {DocID: 1, Freq: 1}}
		if !slices.Equal(idx.Postings["sunscreen"], want) {
			t.Errorf(`idx.Postings["sunscreen"] = %v, want %v`, idx.Postings["sunscreen"], want)
		}
	})

	t.Run("document with no tokens contributes no postings but counts toward N", func(t *testing.T) {
		idx := BuildIndex([]IndexDoc{{ID: 0, Text: "!!!"}})
		if len(idx.Postings) != 0 {
			t.Errorf("Postings = %v, want empty", idx.Postings)
		}
		if idx.N != 1 {
			t.Errorf("N = %d, want 1", idx.N)
		}
		if !slices.Equal(idx.DocLen, []int{0}) {
			t.Errorf("DocLen = %v, want [0]", idx.DocLen)
		}
	})

	t.Run("empty docs slice produces empty index", func(t *testing.T) {
		idx := BuildIndex(nil)
		if len(idx.Postings) != 0 || idx.N != 0 || idx.AvgDocLen != 0 {
			t.Errorf("BuildIndex(nil) = %+v, want zero-value index", idx)
		}
	})

	t.Run("doc lengths and average are computed correctly", func(t *testing.T) {
		idx := BuildIndex([]IndexDoc{
			{ID: 0, Text: "one two three"},  // 3 tokens
			{ID: 1, Text: "four five"},      // 2 tokens
		})
		if !slices.Equal(idx.DocLen, []int{3, 2}) {
			t.Errorf("DocLen = %v, want [3, 2]", idx.DocLen)
		}
		if idx.AvgDocLen != 2.5 {
			t.Errorf("AvgDocLen = %v, want 2.5", idx.AvgDocLen)
		}
		if idx.N != 2 {
			t.Errorf("N = %d, want 2", idx.N)
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

	t.Run("non-contiguous doc IDs panic", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic on non-contiguous doc IDs, got none")
			}
		}()
		BuildIndex([]IndexDoc{{ID: 0, Text: "a"}, {ID: 2, Text: "b"}})
	})
}

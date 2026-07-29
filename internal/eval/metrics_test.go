package eval

import "testing"

func TestPrecisionAtK(t *testing.T) {
	q := Query{Judgments: []Judgment{{DocID: 1, Relevance: 2}, {DocID: 3, Relevance: 1}}}

	t.Run("counts relevant docs within top k", func(t *testing.T) {
		got := PrecisionAtK([]int{1, 2, 3, 4, 5}, q, 5)
		want := 2.0 / 5.0 // docs 1 and 3 are relevant, out of k=5
		if got != want {
			t.Errorf("PrecisionAtK = %v, want %v", got, want)
		}
	})

	t.Run("ignores results beyond k", func(t *testing.T) {
		got := PrecisionAtK([]int{2, 1, 3}, q, 2) // top 2 = [2, 1]; only doc 1 is relevant
		want := 1.0 / 2.0
		if got != want {
			t.Errorf("PrecisionAtK = %v, want %v", got, want)
		}
	})

	t.Run("fewer results than k still divides by k", func(t *testing.T) {
		got := PrecisionAtK([]int{1}, q, 10) // 1 relevant result, but only 1 of 10 slots filled
		want := 1.0 / 10.0
		if got != want {
			t.Errorf("PrecisionAtK = %v, want %v (unfilled ranks count as non-relevant)", got, want)
		}
	})

	t.Run("no relevant docs returns zero", func(t *testing.T) {
		empty := Query{}
		got := PrecisionAtK([]int{1, 2, 3}, empty, 3)
		if got != 0 {
			t.Errorf("PrecisionAtK = %v, want 0", got)
		}
	})
}

func TestReciprocalRank(t *testing.T) {
	q := Query{Judgments: []Judgment{{DocID: 5, Relevance: 1}}}

	t.Run("relevant doc first", func(t *testing.T) {
		got := ReciprocalRank([]int{5, 1, 2}, q)
		if got != 1.0 {
			t.Errorf("ReciprocalRank = %v, want 1.0", got)
		}
	})

	t.Run("relevant doc third", func(t *testing.T) {
		got := ReciprocalRank([]int{1, 2, 5}, q)
		if got != 1.0/3.0 {
			t.Errorf("ReciprocalRank = %v, want %v", got, 1.0/3.0)
		}
	})

	t.Run("no relevant doc present returns zero", func(t *testing.T) {
		got := ReciprocalRank([]int{1, 2, 3}, q)
		if got != 0 {
			t.Errorf("ReciprocalRank = %v, want 0", got)
		}
	})

	t.Run("empty ranked list returns zero", func(t *testing.T) {
		got := ReciprocalRank(nil, q)
		if got != 0 {
			t.Errorf("ReciprocalRank = %v, want 0", got)
		}
	})
}

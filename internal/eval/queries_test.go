package eval

import "testing"

func TestLoadQueries_RealMilestone1Set(t *testing.T) {
	queries, err := LoadQueries("../../data/eval/queries.json")
	if err != nil {
		t.Fatalf("LoadQueries returned error: %v", err)
	}
	if len(queries) != 10 {
		t.Fatalf("got %d queries, want 10", len(queries))
	}
	for i, q := range queries {
		if q.Text == "" {
			t.Errorf("queries[%d] has empty query text", i)
		}
		if len(q.Judgments) == 0 {
			t.Errorf("queries[%d] (%q) has no judgments", i, q.Text)
		}
	}
}

func TestQuery_RelevanceDefaultsToZero(t *testing.T) {
	q := Query{Judgments: []Judgment{{DocID: 7, Relevance: 2}}}
	if got := q.Relevance(7); got != 2 {
		t.Errorf("Relevance(7) = %d, want 2", got)
	}
	if got := q.Relevance(99); got != 0 {
		t.Errorf("Relevance(99) = %d, want 0 (unjudged doc defaults to irrelevant)", got)
	}
}

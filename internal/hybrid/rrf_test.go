package hybrid

import (
	"reflect"
	"testing"
)

// TestFuse_AgreementBeatsASingleSourceTopRank is the property that
// actually justifies RRF over "just use one signal": a passage ranked
// only #2 in each of two rankings should outrank a passage ranked #1 in
// only one of them. Combined, weaker evidence should beat strong,
// single-source evidence.
func TestFuse_AgreementBeatsASingleSourceTopRank(t *testing.T) {
	bm25Ranking := []string{"x", "z"}     // z is #2 lexically
	semanticRanking := []string{"y", "z"} // z is #2 semantically too

	got := Fuse([][]string{bm25Ranking, semanticRanking}, DefaultK)

	if got[0] != "z" {
		t.Errorf("Fuse(...)[0] = %q, want \"z\" (ranked #2 in both lists, should beat #1-in-only-one-list)", got[0])
	}
}

func TestFuse_IDMissingFromOneRankingStillCounted(t *testing.T) {
	got := Fuse([][]string{
		{"a", "b"},
		{"a"}, // b doesn't appear here at all
	}, DefaultK)

	if len(got) != 2 {
		t.Fatalf("Fuse(...) = %v, want 2 IDs (b should still appear, with partial credit)", got)
	}
	if got[0] != "a" {
		t.Errorf("Fuse(...)[0] = %q, want \"a\" (present in both rankings)", got[0])
	}
}

func TestFuse_EmptyRankingsProduceEmptyResult(t *testing.T) {
	got := Fuse([][]string{{}, {}}, DefaultK)
	if len(got) != 0 {
		t.Errorf("Fuse(empty rankings) = %v, want empty", got)
	}
}

func TestFuse_TiedScoresBreakDeterministicallyByID(t *testing.T) {
	// "b" and "c" never co-occur with anything else, so they tie exactly.
	rankings := [][]string{{"b"}, {"c"}}

	first := Fuse(rankings, DefaultK)
	second := Fuse(rankings, DefaultK)

	if !reflect.DeepEqual(first, second) {
		t.Fatalf("Fuse produced different orders across repeated calls with identical input: %v vs %v", first, second)
	}
	if !reflect.DeepEqual(first, []string{"b", "c"}) {
		t.Errorf("Fuse(...) = %v, want [b c] (lexicographic tiebreak)", first)
	}
}

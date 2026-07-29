package corpus

import "testing"

func TestLoadJSONL_SkipsBlankLinesAndAssignsSequentialIDs(t *testing.T) {
	docs, metas, err := LoadJSONL("testdata/small.jsonl")
	if err != nil {
		t.Fatalf("LoadJSONL returned error: %v", err)
	}
	if len(docs) != 2 || len(metas) != 2 {
		t.Fatalf("got %d docs, %d metas, want 2 and 2", len(docs), len(metas))
	}
	if docs[0].ID != 0 || docs[1].ID != 1 {
		t.Errorf("IDs = [%d, %d], want [0, 1]", docs[0].ID, docs[1].ID)
	}
	if docs[0].Text != "a" || docs[1].Text != "b" {
		t.Errorf("Text = [%q, %q], want [%q, %q]", docs[0].Text, docs[1].Text, "a", "b")
	}
	if metas[0].Product != "x" || metas[1].Product != "y" {
		t.Errorf("Product = [%q, %q], want [%q, %q]", metas[0].Product, metas[1].Product, "x", "y")
	}
}

func TestLoadJSONL_MalformedLineReturnsError(t *testing.T) {
	_, _, err := LoadJSONL("testdata/bad.jsonl")
	if err == nil {
		t.Error("expected an error for malformed JSON, got none")
	}
}

func TestLoadJSONL_MissingFileReturnsError(t *testing.T) {
	_, _, err := LoadJSONL("testdata/does-not-exist.jsonl")
	if err == nil {
		t.Error("expected an error for a missing file, got none")
	}
}

func TestLoadJSONL_RealSyntheticSet(t *testing.T) {
	docs, metas, err := LoadJSONL("../../data/synthetic/experiences.jsonl")
	if err != nil {
		t.Fatalf("LoadJSONL returned error: %v", err)
	}
	if len(docs) != len(metas) {
		t.Fatalf("got %d docs but %d metas, want equal", len(docs), len(metas))
	}
	if len(docs) == 0 {
		t.Fatal("got 0 documents, want at least 1")
	}
	for i, m := range metas {
		if m.Source != "synthetic" {
			t.Errorf("metas[%d].Source = %q, want %q", i, m.Source, "synthetic")
		}
		if docs[i].ID != i || m.ID != i {
			t.Errorf("doc/meta at index %d has ID %d/%d, want %d", i, docs[i].ID, m.ID, i)
		}
	}
}

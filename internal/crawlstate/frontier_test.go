package crawlstate

import (
	"path/filepath"
	"testing"
	"time"
)

// drain polls Next() until n URLs have been retrieved or timeout elapses,
// sleeping briefly between not-ready results - mirrors how a real worker
// handles Next() returning ok=false, since this test uses a real clock and
// must respect the frontier's own politeness delay rather than assume
// instant draining.
func drain(t *testing.T, pf *PersistentFrontier, n int) []string {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	var got []string
	for len(got) < n {
		if time.Now().After(deadline) {
			t.Fatalf("timed out after collecting %d/%d URLs: %v", len(got), n, got)
		}
		url, ok := pf.Next()
		if !ok {
			time.Sleep(2 * time.Millisecond)
			continue
		}
		got = append(got, url)
	}
	return got
}

func TestPersistentFrontier_SurvivesReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "frontier.jsonl")
	const delay = 1 * time.Millisecond

	pf1, err := OpenFrontier(path, delay, time.Now)
	if err != nil {
		t.Fatalf("OpenFrontier returned error: %v", err)
	}
	pf1.Add("a.com", "https://a.com/1")
	pf1.Add("a.com", "https://a.com/2")
	pf1.Add("b.com", "https://b.com/1")
	if err := pf1.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	// Simulate a restart: open a fresh PersistentFrontier against the same
	// log, with no in-memory state carried over.
	pf2, err := OpenFrontier(path, delay, time.Now)
	if err != nil {
		t.Fatalf("reopening returned error: %v", err)
	}
	defer pf2.Close()

	got := drain(t, pf2, 3)
	seen := map[string]bool{}
	for _, url := range got {
		seen[url] = true
	}
	for _, want := range []string{"https://a.com/1", "https://a.com/2", "https://b.com/1"} {
		if !seen[want] {
			t.Errorf("replayed frontier is missing %q", want)
		}
	}
}

func TestPersistentFrontier_ContinuesAppendingAfterReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "frontier.jsonl")
	const delay = 1 * time.Millisecond

	pf1, _ := OpenFrontier(path, delay, time.Now)
	pf1.Add("a.com", "https://a.com/1")
	pf1.Close()

	pf2, _ := OpenFrontier(path, delay, time.Now)
	pf2.Add("a.com", "https://a.com/2")
	pf2.Close()

	// A third "restart" should see both entries from both prior runs.
	pf3, err := OpenFrontier(path, delay, time.Now)
	if err != nil {
		t.Fatalf("OpenFrontier returned error: %v", err)
	}
	defer pf3.Close()

	got := drain(t, pf3, 2)
	seen := map[string]bool{}
	for _, url := range got {
		seen[url] = true
	}
	if !seen["https://a.com/1"] || !seen["https://a.com/2"] {
		t.Errorf("expected both entries across two prior sessions, got %v", got)
	}
}

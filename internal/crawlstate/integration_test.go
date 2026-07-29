package crawlstate

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"triedandtold/internal/fetch"
	"triedandtold/internal/robots"
)

// TestResumability_AlreadyFetchedURLIsSkippedAfterRestart is the end-to-end
// proof of this whole design: the frontier log records "enqueue" only,
// never "completed" - so after a simulated crash and restart, the
// naively-replayed frontier still contains the already-fetched URL. It's
// the *separately* persisted dedup registry that correctly causes it to be
// skipped (ErrAlreadySeen) rather than re-fetched, while the genuinely
// unfetched URL still goes through normally.
func TestResumability_AlreadyFetchedURLIsSkippedAfterRestart(t *testing.T) {
	var hitsPage1, hitsPage2 int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/robots.txt":
			http.NotFound(w, r)
		case "/page1":
			atomic.AddInt32(&hitsPage1, 1)
			w.Write([]byte("page one"))
		case "/page2":
			atomic.AddInt32(&hitsPage2, 1)
			w.Write([]byte("page two"))
		}
	}))
	defer srv.Close()

	u, _ := url.Parse(srv.URL)
	host := u.Scheme + "://" + u.Host

	frontierLog := filepath.Join(t.TempDir(), "frontier.jsonl")
	dedupLog := filepath.Join(t.TempDir(), "dedup.jsonl")
	cfg := fetch.Config{MaxAttempts: 2, BaseDelay: time.Millisecond, MaxDelay: 5 * time.Millisecond, MaxRedirects: 3}

	// --- First run: fetch page1 successfully, then "crash" before page2. ---
	pf1, err := OpenFrontier(frontierLog, time.Millisecond, time.Now)
	if err != nil {
		t.Fatalf("OpenFrontier: %v", err)
	}
	pf1.Add(host, srv.URL+"/page1")
	pf1.Add(host, srv.URL+"/page2")

	pr1, err := OpenRegistry(dedupLog, 100, 0.01)
	if err != nil {
		t.Fatalf("OpenRegistry: %v", err)
	}

	fetcher1 := fetch.New(robots.New(srv.Client()), pr1, cfg)

	firstURL, ok := pf1.Next()
	if !ok {
		t.Fatal("expected a URL from the first run")
	}
	if _, err := fetcher1.Fetch(firstURL); err != nil {
		t.Fatalf("first fetch of %q returned error: %v", firstURL, err)
	}

	pf1.Close() // simulate a crash: the second URL is never fetched this run
	pr1.Close()

	// --- Restart: fresh in-memory state, same log files on disk. ---
	pf2, err := OpenFrontier(frontierLog, time.Millisecond, time.Now)
	if err != nil {
		t.Fatalf("reopening frontier: %v", err)
	}
	defer pf2.Close()

	pr2, err := OpenRegistry(dedupLog, 100, 0.01)
	if err != nil {
		t.Fatalf("reopening registry: %v", err)
	}
	defer pr2.Close()

	fetcher2 := fetch.New(robots.New(srv.Client()), pr2, cfg)

	replayed := drain(t, pf2, 2)
	var results []error
	for _, url := range replayed {
		_, err := fetcher2.Fetch(url)
		results = append(results, err)
	}

	var sawAlreadySeen, sawSuccess int
	for _, err := range results {
		switch {
		case errors.Is(err, fetch.ErrAlreadySeen):
			sawAlreadySeen++
		case err == nil:
			sawSuccess++
		default:
			t.Errorf("unexpected error: %v", err)
		}
	}
	if sawAlreadySeen != 1 {
		t.Errorf("got %d ErrAlreadySeen results, want exactly 1 (the already-fetched URL)", sawAlreadySeen)
	}
	if sawSuccess != 1 {
		t.Errorf("got %d successful fetches, want exactly 1 (the genuinely new URL)", sawSuccess)
	}

	if got := atomic.LoadInt32(&hitsPage1); got != 1 {
		t.Errorf("page1 was fetched %d times, want exactly 1 - it must not be re-fetched after restart", got)
	}
	if got := atomic.LoadInt32(&hitsPage2); got != 1 {
		t.Errorf("page2 was fetched %d times, want exactly 1", got)
	}
}

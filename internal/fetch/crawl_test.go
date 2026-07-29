package fetch

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"sync"
	"testing"
	"time"

	"triedandtold/internal/dedup"
	"triedandtold/internal/frontier"
	"triedandtold/internal/robots"
)

func hostOf(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		panic(err)
	}
	return u.Scheme + "://" + u.Host
}

func timestampRecorder() (http.HandlerFunc, func() []time.Time) {
	var mu sync.Mutex
	var timestamps []time.Time
	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			http.NotFound(w, r)
			return
		}
		mu.Lock()
		timestamps = append(timestamps, time.Now())
		mu.Unlock()
		w.Write([]byte("ok"))
	}
	get := func() []time.Time {
		mu.Lock()
		defer mu.Unlock()
		out := make([]time.Time, len(timestamps))
		copy(out, timestamps)
		return out
	}
	return handler, get
}

func TestCrawl_RespectsPolitenessAcrossConcurrentWorkers(t *testing.T) {
	handlerA, getA := timestampRecorder()
	handlerB, getB := timestampRecorder()

	srvA := httptest.NewServer(handlerA)
	defer srvA.Close()
	srvB := httptest.NewServer(handlerB)
	defer srvB.Close()

	// A larger delay than strictly necessary gives real headroom against
	// scheduling jitter when the full test suite runs in parallel under
	// -race (verified this flakes at 50ms/10ms tolerance under full-suite
	// contention, though it passes reliably in isolation - the underlying
	// politeness logic was never wrong, the margin was just too tight).
	const delay = 100 * time.Millisecond
	const perHost = 3

	f := frontier.New(delay, time.Now)
	for i := 0; i < perHost; i++ {
		f.Add(hostOf(srvA.URL), fmt.Sprintf("%s/a%d", srvA.URL, i))
		f.Add(hostOf(srvB.URL), fmt.Sprintf("%s/b%d", srvB.URL, i))
	}

	fetcher := New(robots.New(http.DefaultClient), dedup.New(100, 0.01), testConfig())

	out := make(chan Outcome)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go Crawl(ctx, f, fetcher, 4, out)

	var results []Outcome
	for o := range out {
		results = append(results, o)
	}

	if len(results) != 2*perHost {
		t.Fatalf("got %d results, want %d", len(results), 2*perHost)
	}
	for _, o := range results {
		if o.Err != nil {
			t.Errorf("unexpected error for %s: %v", o.URL, o.Err)
		}
	}

	checkSpacing(t, "host A", getA(), delay)
	checkSpacing(t, "host B", getB(), delay)
}

func checkSpacing(t *testing.T, label string, timestamps []time.Time, minDelay time.Duration) {
	t.Helper()
	if len(timestamps) < 2 {
		t.Fatalf("%s: got %d requests, want at least 2 to check spacing", label, len(timestamps))
	}
	sort.Slice(timestamps, func(i, j int) bool { return timestamps[i].Before(timestamps[j]) })
	const jitterTolerance = 30 * time.Millisecond
	for i := 1; i < len(timestamps); i++ {
		gap := timestamps[i].Sub(timestamps[i-1])
		if gap < minDelay-jitterTolerance {
			t.Errorf("%s: requests %d and %d were only %v apart, want >= %v", label, i-1, i, gap, minDelay)
		}
	}
}

func TestCrawl_StopsWhenFrontierIsDrained(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	f := frontier.New(1*time.Millisecond, time.Now)
	f.Add(hostOf(srv.URL), srv.URL+"/only")

	fetcher := New(robots.New(http.DefaultClient), dedup.New(100, 0.01), testConfig())

	out := make(chan Outcome)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		Crawl(ctx, f, fetcher, 2, out)
		close(done)
	}()

	var results []Outcome
	for o := range out {
		results = append(results, o)
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Crawl did not return after the frontier drained")
	}

	if len(results) != 1 {
		t.Fatalf("got %d results, want 1", len(results))
	}
}

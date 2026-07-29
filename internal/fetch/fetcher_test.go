package fetch

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"triedandtold/internal/dedup"
	"triedandtold/internal/robots"
	"triedandtold/internal/wal"
)

func testConfig() Config {
	return Config{
		MaxAttempts:  3,
		BaseDelay:    1 * time.Millisecond,
		MaxDelay:     10 * time.Millisecond,
		MaxRedirects: 5,
	}
}

func newFetcher(client *http.Client) *Fetcher {
	return New(robots.New(client), dedup.New(100, 0.01), testConfig())
}

func TestFetcher_SuccessfulFetch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte("hello world"))
	}))
	defer srv.Close()

	f := newFetcher(srv.Client())
	result, err := f.Fetch(srv.URL + "/page")
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	if result.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want 200", result.StatusCode)
	}
	if string(result.Body) != "hello world" {
		t.Errorf("Body = %q, want %q", result.Body, "hello world")
	}
	if result.ContentHash == ([32]byte{}) {
		t.Error("ContentHash was left as the zero value")
	}
}

func TestFetcher_ArchivesContentWhenConfigured(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte("archived body"))
	}))
	defer srv.Close()

	logPath := filepath.Join(t.TempDir(), "content.jsonl")
	contentLog, err := wal.Open[ContentRecord](logPath)
	if err != nil {
		t.Fatalf("wal.Open returned error: %v", err)
	}

	f := newFetcher(srv.Client()).WithContentLog(contentLog)
	if _, err := f.Fetch(srv.URL + "/page"); err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	if err := contentLog.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	var records []ContentRecord
	if err := wal.Replay[ContentRecord](logPath, func(r ContentRecord) { records = append(records, r) }); err != nil {
		t.Fatalf("Replay returned error: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("got %d archived records, want 1", len(records))
	}
	if records[0].Body != "archived body" {
		t.Errorf("Body = %q, want %q", records[0].Body, "archived body")
	}
	if records[0].URL != srv.URL+"/page" {
		t.Errorf("URL = %q, want %q", records[0].URL, srv.URL+"/page")
	}
	if records[0].ContentHash == "" {
		t.Error("ContentHash was left empty")
	}
}

func TestFetcher_DuplicateURLIsSkipped(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte("content"))
	}))
	defer srv.Close()

	f := newFetcher(srv.Client())
	if _, err := f.Fetch(srv.URL + "/page"); err != nil {
		t.Fatalf("first fetch returned error: %v", err)
	}
	if _, err := f.Fetch(srv.URL + "/page"); !errors.Is(err, ErrAlreadySeen) {
		t.Errorf("second fetch of the same URL: err = %v, want ErrAlreadySeen", err)
	}
}

func TestFetcher_DisallowedByRobots(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			w.Write([]byte("User-agent: *\nDisallow: /private/\n"))
			return
		}
		w.Write([]byte("secret"))
	}))
	defer srv.Close()

	f := newFetcher(srv.Client())
	if _, err := f.Fetch(srv.URL + "/private/page"); !errors.Is(err, ErrDisallowed) {
		t.Errorf("err = %v, want ErrDisallowed", err)
	}
}

func TestFetcher_RetriesThenSucceeds(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			http.NotFound(w, r)
			return
		}
		if atomic.AddInt32(&calls, 1) < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte("finally"))
	}))
	defer srv.Close()

	f := newFetcher(srv.Client())
	result, err := f.Fetch(srv.URL + "/page")
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	if string(result.Body) != "finally" {
		t.Errorf("Body = %q, want %q", result.Body, "finally")
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Errorf("page was requested %d times, want 3", got)
	}
}

func TestFetcher_GivesUpAfterMaxAttempts(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			http.NotFound(w, r)
			return
		}
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	f := newFetcher(srv.Client())
	if _, err := f.Fetch(srv.URL + "/page"); err == nil {
		t.Fatal("expected an error after exhausting retries, got nil")
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Errorf("page was requested %d times, want exactly MaxAttempts=3", got)
	}
}

func TestFetcher_DoesNotRetry404(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			http.NotFound(w, r)
			return
		}
		atomic.AddInt32(&calls, 1)
		http.NotFound(w, r)
	}))
	defer srv.Close()

	f := newFetcher(srv.Client())
	result, err := f.Fetch(srv.URL + "/missing")
	if err != nil {
		t.Fatalf("Fetch returned error for a plain 404: %v", err)
	}
	if result.StatusCode != http.StatusNotFound {
		t.Errorf("StatusCode = %d, want 404", result.StatusCode)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("page was requested %d times, want exactly 1 (404 must not be retried)", got)
	}
}

func TestFetcher_FollowsRedirectToFinalContent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/robots.txt":
			http.NotFound(w, r)
		case "/old":
			http.Redirect(w, r, "/new", http.StatusMovedPermanently)
		case "/new":
			w.Write([]byte("moved content"))
		}
	}))
	defer srv.Close()

	f := newFetcher(srv.Client())
	result, err := f.Fetch(srv.URL + "/old")
	if err != nil {
		t.Fatalf("Fetch returned error: %v", err)
	}
	if string(result.Body) != "moved content" {
		t.Errorf("Body = %q, want %q", result.Body, "moved content")
	}
	if result.FinalURL != srv.URL+"/new" {
		t.Errorf("FinalURL = %q, want %q", result.FinalURL, srv.URL+"/new")
	}
}

func TestFetcher_RedirectTargetDisallowedByRobotsIsCaught(t *testing.T) {
	// Proves manual redirect-following actually re-checks robots.txt on
	// the target, rather than blindly trusting the client to follow it.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/robots.txt":
			w.Write([]byte("User-agent: *\nDisallow: /private/\n"))
		case "/old":
			http.Redirect(w, r, "/private/secret", http.StatusFound)
		case "/private/secret":
			w.Write([]byte("should never be served"))
		}
	}))
	defer srv.Close()

	f := newFetcher(srv.Client())
	if _, err := f.Fetch(srv.URL + "/old"); !errors.Is(err, ErrDisallowed) {
		t.Errorf("err = %v, want ErrDisallowed (redirect target must be re-checked)", err)
	}
}

func TestFetcher_TooManyRedirectsGivesUp(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/robots.txt" {
			http.NotFound(w, r)
			return
		}
		http.Redirect(w, r, r.URL.Path+"x", http.StatusFound) // never converges
	}))
	defer srv.Close()

	f := newFetcher(srv.Client())
	if _, err := f.Fetch(srv.URL + "/a"); err == nil {
		t.Fatal("expected an error from an infinite redirect chain, got nil")
	}
}

package robots

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestChecker_DisallowedPathIsBlocked(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("User-agent: *\nDisallow: /private/\n"))
	}))
	defer srv.Close()

	c := New(srv.Client())

	allowed, err := c.Allowed(srv.URL + "/private/secret")
	if err != nil {
		t.Fatalf("Allowed returned error: %v", err)
	}
	if allowed {
		t.Error("expected /private/ to be disallowed")
	}

	allowed, err = c.Allowed(srv.URL + "/public/page")
	if err != nil {
		t.Fatalf("Allowed returned error: %v", err)
	}
	if !allowed {
		t.Error("expected /public/page to be allowed")
	}
}

func TestChecker_404MeansAllowAll(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	c := New(srv.Client())

	allowed, err := c.Allowed(srv.URL + "/anything")
	if err != nil {
		t.Fatalf("Allowed returned error: %v", err)
	}
	if !allowed {
		t.Error("expected a 404 robots.txt to mean no restrictions")
	}
}

func TestChecker_ServerErrorFailsClosed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := New(srv.Client())

	allowed, err := c.Allowed(srv.URL + "/anything")
	if err != nil {
		t.Fatalf("Allowed returned error: %v", err)
	}
	if allowed {
		t.Error("expected a robots.txt fetch failure (5xx) to fail closed")
	}
}

func TestChecker_UnreachableHostFailsClosed(t *testing.T) {
	c := New(http.DefaultClient)

	// Port 0 on localhost is never listening - a real connection failure.
	allowed, err := c.Allowed("http://127.0.0.1:0/anything")
	if err != nil {
		t.Fatalf("Allowed returned error: %v", err)
	}
	if allowed {
		t.Error("expected an unreachable host to fail closed")
	}
}

func TestChecker_CachesSuccessfulFetch(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.Write([]byte("User-agent: *\nDisallow: /private/\n"))
	}))
	defer srv.Close()

	c := New(srv.Client())

	for i := 0; i < 5; i++ {
		if _, err := c.Allowed(srv.URL + "/anything"); err != nil {
			t.Fatalf("Allowed returned error: %v", err)
		}
	}

	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Errorf("robots.txt was fetched %d times, want exactly 1 (cached after the first)", got)
	}
}

func TestChecker_FailureIsNotCachedSoLaterCallsRetry(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			w.WriteHeader(http.StatusInternalServerError) // first call fails
			return
		}
		w.Write([]byte("User-agent: *\nDisallow: /private/\n")) // succeeds afterward
	}))
	defer srv.Close()

	c := New(srv.Client())

	allowed, err := c.Allowed(srv.URL + "/anything")
	if err != nil {
		t.Fatalf("Allowed returned error: %v", err)
	}
	if allowed {
		t.Fatal("expected the first (failing) fetch to fail closed")
	}

	allowed, err = c.Allowed(srv.URL + "/anything")
	if err != nil {
		t.Fatalf("Allowed returned error: %v", err)
	}
	if !allowed {
		t.Fatal("expected the second call to retry the fetch and succeed, not stay stuck on the earlier failure")
	}

	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Errorf("robots.txt fetch was attempted %d times, want exactly 2 (failure must not be cached)", got)
	}
}

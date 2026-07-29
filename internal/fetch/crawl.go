package fetch

import (
	"context"
	"sync"
	"time"

	"triedandtold/internal/frontier"
)

// Outcome is one URL's result from a Crawl run - either a successful
// Result, or an error (which may be ErrAlreadySeen/ErrDisallowed rather
// than a real failure - callers should check with errors.Is).
type Outcome struct {
	URL    string
	Result *Result
	Err    error
}

// Crawl runs workers goroutines pulling URLs from f and fetching them with
// fetcher, until the frontier is completely drained or ctx is cancelled.
// It blocks until all workers have stopped, then closes out. Callers
// should drain out concurrently (e.g. in a separate goroutine) rather than
// waiting for Crawl to return first, since out may not be buffered enough
// to hold every result at once.
func Crawl(ctx context.Context, f *frontier.Frontier, fetcher *Fetcher, workers int, out chan<- Outcome) {
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			crawlWorker(ctx, f, fetcher, out)
		}()
	}
	wg.Wait()
	close(out)
}

func crawlWorker(ctx context.Context, f *frontier.Frontier, fetcher *Fetcher, out chan<- Outcome) {
	for {
		if ctx.Err() != nil {
			return
		}

		url, ok := f.Next()
		if ok {
			result, err := fetcher.Fetch(url)
			select {
			case out <- Outcome{URL: url, Result: result, Err: err}:
			case <-ctx.Done():
				return
			}
			continue
		}

		readyAt, hasWork := f.NextReadyAt()
		if !hasWork {
			return // frontier is fully drained - nothing left for this run
		}

		wait := time.Until(readyAt)
		if wait <= 0 {
			continue // already ready; loop back and call Next() again
		}

		timer := time.NewTimer(wait)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			return
		}
	}
}

// Package frontier implements the crawler's URL frontier: per-host FIFO
// queues, scheduled by a min-heap keyed on each host's next-allowed-fetch
// time, per docs/design/06-crawler-frontier.md.
package frontier

import (
	"container/heap"
	"sync"
	"time"
)

// Clock returns the current time. Injected so tests can use a fake,
// instantly-advanceable clock instead of the real wall clock.
type Clock func() time.Time

// hostEntry is one host's position in the readiness heap.
type hostEntry struct {
	host        string
	nextAllowed time.Time
}

// hostHeap is a container/heap.Interface implementation ordering hosts by
// soonest nextAllowed time.
type hostHeap []*hostEntry

func (h hostHeap) Len() int            { return len(h) }
func (h hostHeap) Less(i, j int) bool  { return h[i].nextAllowed.Before(h[j].nextAllowed) }
func (h hostHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *hostHeap) Push(x interface{}) { *h = append(*h, x.(*hostEntry)) }
func (h *hostHeap) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[:n-1]
	return item
}

// Frontier is a politeness-scheduled URL queue: many hosts progress in
// parallel, but each host is only ever offered again after delay has
// passed since it was *last actually fetched* - not just since it last had
// a queue entry, so a host that drains and then gets new work shortly after
// still has to wait out its cooldown.
type Frontier struct {
	mu        sync.Mutex
	delay     time.Duration
	clock     Clock
	ready     hostHeap
	queues    map[string][]string  // host -> pending URLs, FIFO
	inHeap    map[string]bool      // host -> already has an entry in ready
	lastFetch map[string]time.Time // host -> time of its most recent fetch
}

// New creates a Frontier with the given per-host politeness delay.
func New(delay time.Duration, clock Clock) *Frontier {
	return &Frontier{
		delay:     delay,
		clock:     clock,
		queues:    make(map[string][]string),
		inHeap:    make(map[string]bool),
		lastFetch: make(map[string]time.Time),
	}
}

// Add enqueues url under host. If host has no pending heap entry yet, one
// is created - eligible immediately, unless host's last fetch was recent
// enough that the politeness delay hasn't fully elapsed.
func (f *Frontier) Add(host, url string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.queues[host] = append(f.queues[host], url)
	if !f.inHeap[host] {
		heap.Push(&f.ready, &hostEntry{host: host, nextAllowed: f.earliestAllowed(host)})
		f.inHeap[host] = true
	}
}

// earliestAllowed is now, unless host was fetched recently enough that its
// cooldown hasn't elapsed yet.
func (f *Frontier) earliestAllowed(host string) time.Time {
	now := f.clock()
	if last, ok := f.lastFetch[host]; ok {
		if readyAt := last.Add(f.delay); readyAt.After(now) {
			return readyAt
		}
	}
	return now
}

// NextReadyAt returns the time at which the frontier will next have a URL
// ready, and false if the frontier currently has no pending work at all
// (not even anything in cooldown). A caller that gets ok=false from Next
// can sleep exactly until this time instead of busy-polling or sleeping an
// arbitrary fixed interval.
func (f *Frontier) NextReadyAt() (time.Time, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if len(f.ready) == 0 {
		return time.Time{}, false
	}
	return f.ready[0].nextAllowed, true
}

// Next returns the next URL to fetch. ok is false if the frontier is empty
// or every host is still within its politeness cooldown - the caller should
// wait and retry rather than treating this as "done forever."
func (f *Frontier) Next() (url string, ok bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if len(f.ready) == 0 {
		return "", false
	}
	top := f.ready[0]
	if top.nextAllowed.After(f.clock()) {
		return "", false // the soonest host isn't ready yet, so nothing is
	}

	heap.Pop(&f.ready)
	delete(f.inHeap, top.host)
	f.lastFetch[top.host] = f.clock()

	q := f.queues[top.host]
	url, q = q[0], q[1:]

	if len(q) > 0 {
		f.queues[top.host] = q
		heap.Push(&f.ready, &hostEntry{host: top.host, nextAllowed: f.clock().Add(f.delay)})
		f.inHeap[top.host] = true
	} else {
		delete(f.queues, top.host)
	}

	return url, true
}

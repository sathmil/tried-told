package frontier

import (
	"testing"
	"time"
)

// fakeClock lets tests advance time deterministically instead of sleeping.
type fakeClock struct{ now time.Time }

func newFakeClock() *fakeClock {
	return &fakeClock{now: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
}
func (c *fakeClock) Now() time.Time          { return c.now }
func (c *fakeClock) Advance(d time.Duration) { c.now = c.now.Add(d) }

func TestFrontier_SingleHostRespectsDelay(t *testing.T) {
	clock := newFakeClock()
	f := New(10*time.Second, clock.Now)

	f.Add("a.com", "https://a.com/1")
	f.Add("a.com", "https://a.com/2")

	url, ok := f.Next()
	if !ok || url != "https://a.com/1" {
		t.Fatalf("Next() = (%q, %v), want (https://a.com/1, true)", url, ok)
	}

	if _, ok := f.Next(); ok {
		t.Fatal("Next() returned ok=true before the politeness delay elapsed")
	}

	clock.Advance(10 * time.Second)
	url, ok = f.Next()
	if !ok || url != "https://a.com/2" {
		t.Fatalf("Next() after advancing clock = (%q, %v), want (https://a.com/2, true)", url, ok)
	}
}

func TestFrontier_OtherHostsStayBusyDuringCooldown(t *testing.T) {
	clock := newFakeClock()
	f := New(1*time.Minute, clock.Now)

	f.Add("slow.com", "https://slow.com/1")
	f.Add("slow.com", "https://slow.com/2")
	f.Add("fast.com", "https://fast.com/1")

	first, ok := f.Next()
	if !ok {
		t.Fatal("expected a URL")
	}

	// Whichever host was just taken is now in cooldown; some other work
	// must still be immediately available - Next() must succeed with zero
	// clock advancement.
	second, ok := f.Next()
	if !ok {
		t.Fatal("worker went idle even though not every host was in cooldown")
	}
	if first == second {
		t.Fatalf("got the same URL twice: %q", first)
	}
}

func TestFrontier_EmptyReturnsNotOK(t *testing.T) {
	clock := newFakeClock()
	f := New(time.Second, clock.Now)
	if _, ok := f.Next(); ok {
		t.Fatal("Next() on empty frontier returned ok=true")
	}
}

func TestFrontier_DrainedHostGettingNewWorkStillRespectsCooldown(t *testing.T) {
	clock := newFakeClock()
	f := New(10*time.Second, clock.Now)

	f.Add("a.com", "https://a.com/1")
	if _, ok := f.Next(); !ok {
		t.Fatal("expected the first URL")
	}
	// a.com's queue is now empty and it has no heap entry at all.

	clock.Advance(2 * time.Second) // well within the 10s cooldown
	f.Add("a.com", "https://a.com/2")

	if _, ok := f.Next(); ok {
		t.Fatal("new work for a recently-fetched host was offered before its cooldown elapsed")
	}

	clock.Advance(8 * time.Second) // total 10s since the first fetch
	url, ok := f.Next()
	if !ok || url != "https://a.com/2" {
		t.Fatalf("Next() = (%q, %v), want (https://a.com/2, true)", url, ok)
	}
}

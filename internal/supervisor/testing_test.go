package supervisor

import (
	"context"
	"sync"
	"time"
)

// fakeClock is a manual clock used in tests. Goroutines blocked on
// After are released when the test advances the clock past their
// trigger.
type fakeClock struct {
	mu      sync.Mutex
	now     time.Time
	waiters []*fakeTimer
}

type fakeTimer struct {
	trigger time.Time
	ch      chan time.Time
}

func newFakeClock(start time.Time) *fakeClock {
	return &fakeClock{now: start}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) After(d time.Duration) <-chan time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	t := &fakeTimer{trigger: c.now.Add(d), ch: make(chan time.Time, 1)}
	if d <= 0 {
		t.ch <- c.now
		close(t.ch)
		return t.ch
	}
	c.waiters = append(c.waiters, t)
	return t.ch
}

func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	remaining := c.waiters[:0]
	fired := make([]*fakeTimer, 0, len(c.waiters))
	for _, w := range c.waiters {
		if !c.now.Before(w.trigger) {
			fired = append(fired, w)
		} else {
			remaining = append(remaining, w)
		}
	}
	c.waiters = remaining
	now := c.now
	c.mu.Unlock()
	for _, w := range fired {
		w.ch <- now
		close(w.ch)
	}
}

// fakeProbe is a manually-driven MemoryPressureProbe.
type fakeProbe struct {
	mu          sync.Mutex
	subscribers []chan MemoryPressure
}

func newFakeProbe() *fakeProbe {
	return &fakeProbe{}
}

func (p *fakeProbe) Subscribe() <-chan MemoryPressure {
	p.mu.Lock()
	defer p.mu.Unlock()
	ch := make(chan MemoryPressure, 4)
	p.subscribers = append(p.subscribers, ch)
	return ch
}

func (p *fakeProbe) Run(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}

func (p *fakeProbe) broadcast(mp MemoryPressure) {
	p.mu.Lock()
	subs := append([]chan MemoryPressure(nil), p.subscribers...)
	p.mu.Unlock()
	for _, s := range subs {
		select {
		case s <- mp:
		default:
		}
	}
}

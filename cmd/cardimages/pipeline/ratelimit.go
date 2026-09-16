package pipeline

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

var errNonPositiveInterval = errors.New("rate-limit interval must be positive")

// Limiter paces every caller to at most one event per interval, with no burst
// allowance. It is the courtesy owed to a free API, and it is deliberately the
// most conservative shape: a token bucket would let a cold start fire its
// whole capacity at once, which is exactly the spike a rate limit exists to
// stop.
//
// golang.org/x/time/rate is the obvious alternative and was not used: its Wait
// sleeps on real timers with no seam for a fake clock, so the pacing could
// only be tested by actually waiting.
type Limiter struct {
	clock    Clock
	interval time.Duration

	mu sync.Mutex
	// next is the earliest instant at which a further event may start. Every
	// reservation moves it forward by one interval, so N concurrent callers
	// take N distinct slots rather than all waking at once.
	next time.Time
}

// NewLimiter returns a limiter that admits one event per interval.
func NewLimiter(clock Clock, interval time.Duration) (*Limiter, error) {
	if clock == nil {
		return nil, fmt.Errorf("creating rate limiter: %w", errNilClock)
	}
	if interval <= 0 {
		return nil, fmt.Errorf("creating rate limiter: %w", errNonPositiveInterval)
	}
	return &Limiter{clock: clock, interval: interval}, nil
}

// Wait blocks until the caller's slot arrives, or returns the context's error
// if the run is cancelled first.
func (l *Limiter) Wait(ctx context.Context) error {
	wait := l.reserve()
	if wait <= 0 {
		return nil
	}
	if err := l.clock.Sleep(ctx, wait); err != nil {
		return fmt.Errorf("waiting for a rate-limit slot: %w", err)
	}
	return nil
}

// reserve claims the next slot and reports how long the caller must wait for
// it. The lock covers only the arithmetic and never the sleep — holding it
// across the wait would serialise the callers twice over.
func (l *Limiter) reserve() time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.clock.Now()
	if l.next.Before(now) {
		// Idle long enough that the last reservation has passed: start from
		// now rather than letting unused slots accumulate into a burst.
		l.next = now
	}
	slot := l.next
	l.next = slot.Add(l.interval)
	return slot.Sub(now)
}

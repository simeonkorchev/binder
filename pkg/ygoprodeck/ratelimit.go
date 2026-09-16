package ygoprodeck

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Clock is the time source a RateLimiter paces against. Production uses the
// wall clock; a test supplies a fake one, which is how the limiter is proven
// to limit without a spec that actually sleeps.
type Clock interface {
	// Now reports the current time.
	Now() time.Time
	// Sleep blocks for d, or returns early with ctx's error if the context is
	// done first.
	Sleep(ctx context.Context, d time.Duration) error
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

func (systemClock) Sleep(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return fmt.Errorf("waiting out the rate limit: %w", ctx.Err())
	case <-timer.C:
		return nil
	}
}

// RateLimiter paces calls to a fixed number per second. Every Wait takes the
// next slot and blocks until it starts, so N concurrent callers are spread
// over N slots rather than all being let through at once.
type RateLimiter struct {
	interval time.Duration
	clock    Clock

	mu   sync.Mutex
	next time.Time // the earliest time the next call may start
}

// NewRateLimiter returns a limiter allowing perSecond calls per second.
// A perSecond of zero or less means DefaultRequestsPerSecond; a nil clock
// means the wall clock.
func NewRateLimiter(perSecond float64, clock Clock) *RateLimiter {
	if perSecond <= 0 {
		perSecond = DefaultRequestsPerSecond
	}
	if clock == nil {
		clock = systemClock{}
	}

	return &RateLimiter{
		interval: time.Duration(float64(time.Second) / perSecond),
		clock:    clock,
	}
}

// Wait blocks until the caller's slot comes up. It returns the context's error
// if the context is cancelled while waiting.
func (l *RateLimiter) Wait(ctx context.Context) error {
	wait := l.reserve()
	if wait <= 0 {
		return nil
	}

	return l.clock.Sleep(ctx, wait)
}

// reserve claims the next slot and reports how long the caller must wait for
// it. The lock is released before the caller sleeps — holding it across the
// sleep would make every waiter queue on the mutex instead of on its own slot.
func (l *RateLimiter) reserve() time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.clock.Now()
	if l.next.Before(now) {
		l.next = now
	}
	wait := l.next.Sub(now)
	l.next = l.next.Add(l.interval)

	return wait
}

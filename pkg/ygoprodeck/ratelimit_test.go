package ygoprodeck_test

import (
	"context"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/simeonkorchev/binder/pkg/ygoprodeck"
)

// fakeClock is virtual time: Sleep advances the clock instead of blocking, so
// these specs prove the limiter paces calls without a spec that actually waits.
type fakeClock struct {
	mu    sync.Mutex
	now   time.Time
	slept []time.Duration
}

func newFakeClock() *fakeClock {
	return &fakeClock{now: time.Date(2026, time.September, 16, 12, 0, 0, 0, time.UTC)}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.now
}

func (c *fakeClock) Sleep(ctx context.Context, d time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.slept = append(c.slept, d)
	c.now = c.now.Add(d)

	return nil
}

func (c *fakeClock) sleeps() []time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()

	return append([]time.Duration(nil), c.slept...)
}

func (c *fakeClock) totalSlept() time.Duration {
	var total time.Duration
	for _, d := range c.sleeps() {
		total += d
	}

	return total
}

// advance moves virtual time forward without going through the limiter, which
// is how "the caller was busy for a while" is expressed.
func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// defaultInterval is one slot at DefaultRequestsPerSecond.
const defaultInterval = time.Second / ygoprodeck.DefaultRequestsPerSecond

var _ = Describe("RateLimiter", func() {
	var (
		clock   *fakeClock
		limiter *ygoprodeck.RateLimiter
		ctx     context.Context
	)

	BeforeEach(func() {
		clock = newFakeClock()
		ctx = context.Background()
	})

	When("calls are made back to back at the documented ceiling", func() {
		BeforeEach(func() {
			limiter = ygoprodeck.NewRateLimiter(ygoprodeck.DefaultRequestsPerSecond, clock)
		})

		JustBeforeEach(func() {
			for range 5 {
				Expect(limiter.Wait(ctx)).To(Succeed())
			}
		})

		It("lets the first call straight through and paces the rest one 20th of a second apart", func() {
			Expect(clock.sleeps()).To(Equal([]time.Duration{
				defaultInterval, defaultInterval,
				defaultInterval, defaultInterval,
			}))
		})

		It("spreads five calls over four intervals, never faster than 20 a second", func() {
			Expect(clock.totalSlept()).To(Equal(4 * defaultInterval))
		})
	})

	When("the ceiling is configured lower", func() {
		BeforeEach(func() {
			limiter = ygoprodeck.NewRateLimiter(2, clock)
		})

		It("honours the configured rate, not the default", func() {
			Expect(limiter.Wait(ctx)).To(Succeed())
			Expect(limiter.Wait(ctx)).To(Succeed())

			Expect(clock.sleeps()).To(Equal([]time.Duration{time.Second / 2}))
		})
	})

	When("the ceiling is zero or negative", func() {
		BeforeEach(func() {
			limiter = ygoprodeck.NewRateLimiter(0, clock)
		})

		It("falls back to the documented ceiling", func() {
			Expect(limiter.Wait(ctx)).To(Succeed())
			Expect(limiter.Wait(ctx)).To(Succeed())

			Expect(clock.sleeps()).To(Equal([]time.Duration{defaultInterval}))
		})
	})

	When("the caller was busy for longer than the interval", func() {
		BeforeEach(func() {
			limiter = ygoprodeck.NewRateLimiter(ygoprodeck.DefaultRequestsPerSecond, clock)
		})

		It("does not sleep, and does not bank the unused time", func() {
			Expect(limiter.Wait(ctx)).To(Succeed())
			clock.advance(time.Second)

			Expect(limiter.Wait(ctx)).To(Succeed())
			Expect(limiter.Wait(ctx)).To(Succeed())

			Expect(clock.sleeps()).To(Equal([]time.Duration{defaultInterval}))
		})
	})

	When("callers arrive concurrently", func() {
		const callers = 4

		BeforeEach(func() {
			limiter = ygoprodeck.NewRateLimiter(ygoprodeck.DefaultRequestsPerSecond, clock)
		})

		It("gives each one its own slot instead of letting them all through", func() {
			start := clock.Now()

			var wg sync.WaitGroup
			for range callers {
				wg.Add(1)
				go func() {
					defer GinkgoRecover()
					defer wg.Done()
					Expect(limiter.Wait(ctx)).To(Succeed())
				}()
			}
			wg.Wait()

			// Which goroutine gets which slot is a race, so the durations they
			// individually slept are not fixed. That every one of them took a
			// slot is: only the first is free, and a caller arriving after all
			// four have gone can start no earlier than four intervals in.
			Expect(clock.sleeps()).To(HaveLen(callers - 1))
			Expect(limiter.Wait(ctx)).To(Succeed())
			Expect(clock.Now().Sub(start)).To(Equal(callers * defaultInterval))
		})
	})

	When("the context is cancelled while a caller is being paced", func() {
		BeforeEach(func() {
			limiter = ygoprodeck.NewRateLimiter(ygoprodeck.DefaultRequestsPerSecond, clock)
		})

		It("gives up waiting and reports why", func() {
			cancelled, cancel := context.WithCancel(context.Background())
			cancel()

			Expect(limiter.Wait(cancelled)).To(Succeed()) // the first slot is free
			Expect(limiter.Wait(cancelled)).To(MatchError(context.Canceled))
		})
	})
})

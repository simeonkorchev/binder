package pipeline_test

import (
	"context"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/simeonkorchev/binder/cmd/cardimages/pipeline"
	"github.com/simeonkorchev/binder/cmd/cardimages/pipeline/pipelinefakes"
	"golang.org/x/sync/errgroup"
)

// Every spec here drives a fake clock: the limiter is proved to limit by the
// waits it asks for, not by a suite that actually waits. A spec that slept for
// real would take seconds to prove a millisecond and would still only prove it
// on a fast machine.
var _ = Describe("Limiter", func() {
	const interval = 200 * time.Millisecond

	var (
		fakeClock *pipelinefakes.FakeClock
		start     time.Time
		subject   *pipeline.Limiter
		mu        sync.Mutex
		now       time.Time
	)

	BeforeEach(func() {
		start = time.Date(2026, time.September, 16, 12, 0, 0, 0, time.UTC)
		now = start
		fakeClock = new(pipelinefakes.FakeClock)
		fakeClock.NowStub = func() time.Time {
			mu.Lock()
			defer mu.Unlock()

			return now
		}

		var err error
		subject, err = pipeline.NewLimiter(fakeClock, interval)
		Expect(err).NotTo(HaveOccurred())
	})

	When("time passes while the callers wait", func() {
		BeforeEach(func() {
			// A sleeping caller really does move the clock on, which is what
			// makes the next reservation land one interval later and not two.
			fakeClock.SleepStub = func(_ context.Context, d time.Duration) error {
				mu.Lock()
				defer mu.Unlock()
				now = now.Add(d)

				return nil
			}
		})

		It("lets the first caller straight through", func() {
			Expect(subject.Wait(context.Background())).To(Succeed())
			Expect(fakeClock.SleepCallCount()).To(BeZero())
		})

		It("holds every later caller for exactly one interval", func() {
			for range 3 {
				Expect(subject.Wait(context.Background())).To(Succeed())
			}

			Expect(sleptFor(fakeClock)).To(Equal([]time.Duration{interval, interval}))
			Expect(now.Sub(start)).To(Equal(2*interval), "three events at 5/s take two intervals")
		})

		It("does not bank unused slots into a burst after an idle spell", func() {
			Expect(subject.Wait(context.Background())).To(Succeed())

			mu.Lock()
			now = now.Add(time.Hour)
			mu.Unlock()

			Expect(subject.Wait(context.Background())).To(Succeed())
			Expect(fakeClock.SleepCallCount()).To(BeZero(), "an idle hour is not an hour of credit")
		})
	})

	When("several workers ask at once", func() {
		BeforeEach(func() {
			// The clock stands still, so each caller's wait is the distance to
			// the slot it reserved and the set of waits is the schedule.
			fakeClock.SleepReturns(nil)
		})

		It("gives each of them a slot of its own", func() {
			group := errgroup.Group{}
			for range 4 {
				group.Go(func() error { return subject.Wait(context.Background()) })
			}
			Expect(group.Wait()).To(Succeed())

			// One caller goes now, the other three are spread one interval
			// apart: four workers cannot turn into four simultaneous requests.
			Expect(sleptFor(fakeClock)).To(ConsistOf(interval, 2*interval, 3*interval))
		})
	})

	When("the run is cancelled while a caller is waiting", func() {
		BeforeEach(func() {
			fakeClock.SleepReturns(context.Canceled)
		})

		It("gives up its slot and reports why", func() {
			Expect(subject.Wait(context.Background())).To(Succeed())

			err := subject.Wait(context.Background())
			Expect(err).To(MatchError(context.Canceled))
		})
	})
})

var _ = Describe("NewLimiter", func() {
	It("refuses a nil clock, because a limiter without one cannot pace anything", func() {
		_, err := pipeline.NewLimiter(nil, time.Second)
		Expect(err).To(MatchError(ContainSubstring("clock is nil")))
	})

	It("refuses a non-positive interval rather than admitting everyone at once", func() {
		_, err := pipeline.NewLimiter(new(pipelinefakes.FakeClock), 0)
		Expect(err).To(MatchError(ContainSubstring("interval must be positive")))
	})
})

func sleptFor(clock *pipelinefakes.FakeClock) []time.Duration {
	durations := make([]time.Duration, 0, clock.SleepCallCount())
	for call := range clock.SleepCallCount() {
		_, d := clock.SleepArgsForCall(call)
		durations = append(durations, d)
	}

	return durations
}

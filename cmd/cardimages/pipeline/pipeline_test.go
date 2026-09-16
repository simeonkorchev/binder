package pipeline_test

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/simeonkorchev/binder/cmd/cardimages/model"
	"github.com/simeonkorchev/binder/cmd/cardimages/pipeline"
	"github.com/simeonkorchev/binder/cmd/cardimages/pipeline/pipelinefakes"
)

// Nothing in this suite opens a socket: images.ygoprodeck.com is answered with
// 403 by the egress proxy in this repository's containers, so the fetcher is a
// fake and the clock is a fake. No real image was ever downloaded.
var _ = Describe("Pipeline", func() {
	var (
		fakeStore   *pipelinefakes.FakeStore
		fakeFetcher *pipelinefakes.FakeFetcher
		fakeStorage *pipelinefakes.FakeStorage
		fakeClock   *pipelinefakes.FakeClock

		ctx    context.Context
		result pipeline.Result
		err    error
	)

	// Three cards that have no image yet — the rows a run selects.
	first := model.PendingCard{ID: uuid.New(), YGOProDeckID: 100}
	second := model.PendingCard{ID: uuid.New(), YGOProDeckID: 200}
	third := model.PendingCard{ID: uuid.New(), YGOProDeckID: 300}

	BeforeEach(func() {
		ctx = context.Background()

		fakeStore = new(pipelinefakes.FakeStore)
		fakeFetcher = new(pipelinefakes.FakeFetcher)
		fakeStorage = new(pipelinefakes.FakeStorage)
		fakeClock = new(pipelinefakes.FakeClock)

		// One batch, then the drained work set that ends the run.
		fakeStore.CardsNeedingImageReturns(nil, nil)
		fakeStore.CardsNeedingImageReturnsOnCall(0, []model.PendingCard{first, second, third}, nil)
		fakeFetcher.FetchReturns(model.Image{Body: []byte("jpeg bytes"), ContentType: "image/jpeg"}, nil)
	})

	JustBeforeEach(func() {
		subject, newErr := pipeline.New(pipeline.Config{
			Store:       fakeStore,
			Fetcher:     fakeFetcher,
			Storage:     fakeStorage,
			Clock:       fakeClock,
			Concurrency: 2,
			Interval:    time.Millisecond,
			BatchSize:   10,
		})
		Expect(newErr).NotTo(HaveOccurred())

		result, err = subject.Run(ctx)
	})

	When("no card has an image yet", func() {
		It("stores one image per card and reports them", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Stored).To(Equal(3))
			Expect(result.Failed()).To(BeZero())
			Expect(result.Considered()).To(Equal(3))
		})

		It("keys each object on the upstream id and the served content type", func() {
			Expect(putKeys(fakeStorage)).To(ConsistOf("cards/100.jpg", "cards/200.jpg", "cards/300.jpg"))
		})

		It("records the key against the card, which is what drains the work set", func() {
			Expect(recordedKeys(fakeStore)).To(Equal(map[uuid.UUID]string{
				first.ID:  "cards/100.jpg",
				second.ID: "cards/200.jpg",
				third.ID:  "cards/300.jpg",
			}))
		})

		It("walks the work set with a cursor rather than asking for the same batch twice", func() {
			Expect(fakeStore.CardsNeedingImageCallCount()).To(Equal(2))

			_, firstCursor, limit := fakeStore.CardsNeedingImageArgsForCall(0)
			Expect(firstCursor).To(BeZero())
			Expect(limit).To(Equal(10))

			_, secondCursor, _ := fakeStore.CardsNeedingImageArgsForCall(1)
			Expect(secondCursor).To(Equal(third.YGOProDeckID), "the cursor is the last card of the batch")
		})
	})

	When("an earlier run already stored most of the images", func() {
		BeforeEach(func() {
			// The store's predicate is image_object_key IS NULL, so a resumed
			// run is simply handed a shorter list — proved end to end in
			// cmd/cardimages/store.
			fakeStore.CardsNeedingImageReturnsOnCall(0, []model.PendingCard{second}, nil)
		})

		It("fetches only what is left", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Stored).To(Equal(1))
			Expect(fakeFetcher.FetchCallCount()).To(Equal(1))

			_, fetched := fakeFetcher.FetchArgsForCall(0)
			Expect(fetched).To(Equal(second.YGOProDeckID))
		})
	})

	When("every card already has an image", func() {
		BeforeEach(func() {
			fakeStore.CardsNeedingImageReturnsOnCall(0, nil, nil)
		})

		It("is a successful run that did nothing", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Considered()).To(BeZero())
			Expect(fakeFetcher.FetchCallCount()).To(BeZero())
		})
	})

	When("upstream has no image for one card", func() {
		BeforeEach(func() {
			fakeFetcher.FetchStub = func(_ context.Context, id int64) (model.Image, error) {
				if id == second.YGOProDeckID {
					return model.Image{}, model.ErrImageNotFound
				}

				return model.Image{Body: []byte("jpeg bytes"), ContentType: "image/jpeg"}, nil
			}
		})

		It("counts it apart from a real failure and finishes the other cards", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Stored).To(Equal(2))
			Expect(result.Failures).To(Equal(map[pipeline.FailureReason]int{pipeline.FailureNotFound: 1}))
		})

		It("leaves that card's key unset, so the next run picks it up again", func() {
			Expect(recordedKeys(fakeStore)).NotTo(HaveKey(second.ID))
		})
	})

	When("a fetch fails part-way through the run", func() {
		BeforeEach(func() {
			fakeFetcher.FetchStub = func(_ context.Context, id int64) (model.Image, error) {
				if id == first.YGOProDeckID {
					return model.Image{}, errUpstream
				}

				return model.Image{Body: []byte("jpeg bytes"), ContentType: "image/jpeg"}, nil
			}
		})

		It("does not abort the run — the other cards are still stored", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Stored).To(Equal(2))
			Expect(result.Failures).To(Equal(map[pipeline.FailureReason]int{pipeline.FailureFetch: 1}))
		})

		It("leaves the failed card for the next run", func() {
			Expect(recordedKeys(fakeStore)).NotTo(HaveKey(first.ID))
		})
	})

	When("upstream serves bytes we will not label", func() {
		BeforeEach(func() {
			fakeFetcher.FetchReturnsOnCall(0, model.Image{Body: []byte("?"), ContentType: "text/html"}, nil)
		})

		It("writes no object rather than one whose extension lies", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Failures).To(HaveKeyWithValue(pipeline.FailureUnsupportedType, 1))
			Expect(fakeStorage.PutCallCount()).To(Equal(2), "the other two cards are unaffected")
		})
	})

	When("the object store refuses a write", func() {
		BeforeEach(func() {
			fakeStorage.PutReturnsOnCall(0, errUpstream)
		})

		It("counts it and never records a key for an object that is not there", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Failures).To(HaveKeyWithValue(pipeline.FailureStore, 1))
			Expect(fakeStore.SetImageObjectKeyCallCount()).To(Equal(2))
		})
	})

	When("the key cannot be written back", func() {
		BeforeEach(func() {
			fakeStore.SetImageObjectKeyReturnsOnCall(0, errUpstream)
		})

		It("counts it and leaves the card in the work set, the object being overwritten next time", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Stored).To(Equal(2))
			Expect(result.Failures).To(HaveKeyWithValue(pipeline.FailureRecord, 1))
		})
	})

	When("listing the work set fails", func() {
		BeforeEach(func() {
			fakeStore.CardsNeedingImageReturnsOnCall(0, nil, errUpstream)
		})

		It("stops the run: without the work set there is nothing to do", func() {
			Expect(err).To(MatchError(errUpstream))
			Expect(fakeFetcher.FetchCallCount()).To(BeZero())
		})
	})

	When("the run is cancelled", func() {
		BeforeEach(func() {
			cancelled, cancel := context.WithCancel(context.Background())
			cancel()
			ctx = cancelled

			fakeFetcher.FetchReturns(model.Image{}, errUpstream)
		})

		It("stops rather than counting every remaining card as a failure", func() {
			Expect(err).To(MatchError(context.Canceled))
		})
	})
})

var _ = Describe("New", func() {
	var (
		cfg pipeline.Config
		err error
	)

	BeforeEach(func() {
		cfg = pipeline.Config{
			Store:       new(pipelinefakes.FakeStore),
			Fetcher:     new(pipelinefakes.FakeFetcher),
			Storage:     new(pipelinefakes.FakeStorage),
			Clock:       new(pipelinefakes.FakeClock),
			Concurrency: 1,
			Interval:    time.Millisecond,
			BatchSize:   1,
		}
	})

	JustBeforeEach(func() {
		_, err = pipeline.New(cfg)
	})

	When("the configuration is complete", func() {
		It("builds the pipeline", func() {
			Expect(err).NotTo(HaveOccurred())
		})
	})

	DescribeTable("refusing to start on a misconfigured run",
		func(breakIt func(*pipeline.Config), want string) {
			broken := pipeline.Config{
				Store:       new(pipelinefakes.FakeStore),
				Fetcher:     new(pipelinefakes.FakeFetcher),
				Storage:     new(pipelinefakes.FakeStorage),
				Clock:       new(pipelinefakes.FakeClock),
				Concurrency: 1,
				Interval:    time.Millisecond,
				BatchSize:   1,
			}
			breakIt(&broken)

			_, newErr := pipeline.New(broken)

			Expect(newErr).To(MatchError(ContainSubstring(want)))
		},
		Entry("no store", func(c *pipeline.Config) { c.Store = nil }, "store is nil"),
		Entry("no fetcher", func(c *pipeline.Config) { c.Fetcher = nil }, "fetcher is nil"),
		Entry("no storage", func(c *pipeline.Config) { c.Storage = nil }, "storage is nil"),
		Entry("no clock", func(c *pipeline.Config) { c.Clock = nil }, "clock is nil"),
		Entry("no concurrency", func(c *pipeline.Config) { c.Concurrency = 0 }, "concurrency must be positive"),
		Entry("no batch size", func(c *pipeline.Config) { c.BatchSize = 0 }, "batch size must be positive"),
		Entry("no interval", func(c *pipeline.Config) { c.Interval = 0 }, "interval must be positive"),
	)
})

var errUpstream = errors.New("upstream is unhappy")

func putKeys(storage *pipelinefakes.FakeStorage) []string {
	keys := make([]string, 0, storage.PutCallCount())
	for call := range storage.PutCallCount() {
		_, key, _, _ := storage.PutArgsForCall(call)
		keys = append(keys, key)
	}

	return keys
}

func recordedKeys(store *pipelinefakes.FakeStore) map[uuid.UUID]string {
	keys := make(map[uuid.UUID]string, store.SetImageObjectKeyCallCount())
	for call := range store.SetImageObjectKeyCallCount() {
		_, cardID, key := store.SetImageObjectKeyArgsForCall(call)
		keys[cardID] = key
	}

	return keys
}

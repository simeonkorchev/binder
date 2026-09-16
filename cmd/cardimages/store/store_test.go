package store_test

import (
	"context"
	"strconv"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/simeonkorchev/binder/cmd/cardimages/model"
	"github.com/simeonkorchev/binder/cmd/cardimages/store"
)

// These specs are where the image pipeline's resumability is actually proved.
// The work set is "image_object_key IS NULL": a card that was never fetched or
// whose fetch failed is still in it on the next run, and one whose key was
// recorded never is. Nothing else in the pipeline remembers where a killed run
// stopped, so if this predicate is wrong the whole design is.
var _ = Describe("Store", func() {
	const noCursor = 0

	var (
		ctx     context.Context
		subject *store.Store
	)

	BeforeEach(func() {
		ctx = context.Background()
		subject = store.NewStore(testDB)

		_, err := testDB.ExecContext(ctx, `TRUNCATE cards CASCADE`)
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("CardsNeedingImage", func() {
		var (
			cards []model.PendingCard
			err   error
			after int64
			limit int
		)

		BeforeEach(func() {
			after = noCursor
			limit = 10
		})

		JustBeforeEach(func() {
			cards, err = subject.CardsNeedingImage(ctx, after, limit)
		})

		When("no card has an image yet", func() {
			BeforeEach(func() {
				seedCard(ctx, 300, nil)
				seedCard(ctx, 100, nil)
				seedCard(ctx, 200, nil)
			})

			It("returns every card, in upstream-id order so the cursor can walk", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(upstreamIDs(cards)).To(Equal([]int64{100, 200, 300}))
			})

			It("carries the id a key is recorded against", func() {
				Expect(cards[0].ID).NotTo(Equal(uuid.Nil))
			})
		})

		When("an earlier run already stored some images", func() {
			BeforeEach(func() {
				seedCard(ctx, 100, ptr("cards/100.jpg"))
				seedCard(ctx, 200, nil)
				seedCard(ctx, 300, ptr("cards/300.jpg"))
				seedCard(ctx, 400, nil)
			})

			It("returns only the cards still missing one, so a resumed run re-fetches nothing", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(upstreamIDs(cards)).To(Equal([]int64{200, 400}))
			})
		})

		When("every card has an image", func() {
			BeforeEach(func() {
				seedCard(ctx, 100, ptr("cards/100.jpg"))
			})

			It("is an empty slice and not an error — the drained backlog is the steady state", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(cards).To(BeEmpty())
			})
		})

		When("there are no cards at all", func() {
			It("is an empty slice and not an error", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(cards).To(BeEmpty())
			})
		})

		When("the cursor has already passed some cards", func() {
			BeforeEach(func() {
				seedCard(ctx, 100, nil)
				seedCard(ctx, 200, nil)
				seedCard(ctx, 300, nil)
				after = 200
			})

			It("starts strictly after it, so a failed card is not retried within the run", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(upstreamIDs(cards)).To(Equal([]int64{300}))
			})
		})

		When("more cards need an image than the batch holds", func() {
			BeforeEach(func() {
				seedCard(ctx, 100, nil)
				seedCard(ctx, 200, nil)
				seedCard(ctx, 300, nil)
				limit = 2
			})

			It("returns at most the batch size", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(upstreamIDs(cards)).To(Equal([]int64{100, 200}))
			})
		})
	})

	Describe("SetImageObjectKey", func() {
		var (
			cardID uuid.UUID
			err    error
		)

		JustBeforeEach(func() {
			err = subject.SetImageObjectKey(ctx, cardID, "cards/100.jpg")
		})

		When("the card is there", func() {
			BeforeEach(func() {
				cardID = seedCard(ctx, 100, nil)
			})

			It("records the key", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(imageKeyOf(ctx, cardID)).To(Equal("cards/100.jpg"))
			})

			It("takes the card out of the work set for good", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(subject.CardsNeedingImage(ctx, noCursor, 10)).To(BeEmpty())
			})
		})

		When("the card was deleted while the run was in flight", func() {
			BeforeEach(func() {
				cardID = uuid.New()
			})

			It("says so, rather than reporting a store that went nowhere", func() {
				Expect(err).To(MatchError(store.ErrCardNotFound))
			})
		})
	})
})

func ptr(s string) *string { return &s }

// seedCard inserts one card and returns its id. The key is a pointer because
// NULL — no image yet — is the state the whole pipeline keys on.
func seedCard(ctx context.Context, upstreamID int64, imageKey *string) uuid.UUID {
	id := uuid.New()
	_, err := testDB.ExecContext(ctx,
		`INSERT INTO cards (id, ygoprodeck_id, name, image_object_key) VALUES ($1, $2, $3, $4)`,
		id, upstreamID, "Card "+strconv.FormatInt(upstreamID, 10), imageKey,
	)
	Expect(err).NotTo(HaveOccurred())

	return id
}

func imageKeyOf(ctx context.Context, cardID uuid.UUID) string {
	var key string
	Expect(testDB.GetContext(ctx, &key, `SELECT image_object_key FROM cards WHERE id = $1`, cardID)).To(Succeed())

	return key
}

func upstreamIDs(cards []model.PendingCard) []int64 {
	ids := make([]int64, 0, len(cards))
	for _, card := range cards {
		ids = append(ids, card.YGOProDeckID)
	}

	return ids
}

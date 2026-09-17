package store_test

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/simeonkorchev/binder/internal/binder/model"
	"github.com/simeonkorchev/binder/internal/binder/store"
	cardmodel "github.com/simeonkorchev/binder/internal/card/model"
	"github.com/simeonkorchev/binder/internal/dataerror"
)

var _ = Describe("InsertSlots", func() {
	var ctx = context.Background()

	var (
		subject    *store.Store
		binderID   uuid.UUID
		cardID     uuid.UUID
		printingID uuid.UUID
		batch      []model.Slot
		stored     []model.Slot
		err        error
	)

	BeforeEach(func() {
		subject = store.NewStore(testDB)
		binderID = seedBinder(seedUser(), "Main binder")
		cardID = seedCard("Dark Magician")
		printingID = seedPrinting(cardID, "LOB-EN005", "Ultra Rare")
		batch = []model.Slot{namedSlot(binderID, cardID, 0), namedSlot(binderID, cardID, 1), namedSlot(binderID, cardID, 2)}
	})

	AfterEach(truncateAll)

	JustBeforeEach(func() {
		stored, err = subject.InsertSlots(ctx, batch)
	})

	When("every card in the batch is good", func() {
		It("writes them all, in position order, with the timestamps the database assigned", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(stored).To(HaveLen(3))

			for i, slot := range stored {
				Expect(slot.ID).To(Equal(batch[i].ID))
				Expect(slot.BinderID).To(Equal(binderID))
				Expect(slot.Position).To(Equal(i))
				Expect(slot.CardID).To(Equal(cardID))
				Expect(slot.CardPrintingID).To(BeNil())
				Expect(slot.SetResolution).To(Equal(cardmodel.SetResolutionByName))
				Expect(slot.CreatedAt).NotTo(BeZero())
				Expect(slot.UpdatedAt).NotTo(BeZero())
			}
		})

		It("leaves the binder dense from zero", func() {
			Expect(positionsOf(binderID)).To(HaveLen(3))
		})

		Context("and one of them is a printing the user picked", func() {
			BeforeEach(func() {
				batch[1].CardPrintingID = &printingID
				batch[1].SetResolution = cardmodel.SetResolutionManual
			})

			// The rung db/migrations/005_manual_set_resolution.sql added. It is
			// written through the real schema so the Go constant and the enum
			// cannot drift apart.
			It("stores it as manual, with the printing the user chose", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(stored[1].SetResolution).To(Equal(cardmodel.SetResolutionManual))
				Expect(stored[1].CardPrintingID).To(HaveValue(Equal(printingID)))
			})
		})
	})

	When("the batch is empty", func() {
		BeforeEach(func() {
			batch = nil
		})

		// 000-principles.md section 8b: a write of nothing wrote nothing, which
		// is not a failure.
		It("writes nothing and does not call it an error", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(stored).To(BeEmpty())
			Expect(stored).NotTo(BeNil())
		})
	})

	// The reason this method exists. A batch is one statement, so a card the
	// database refuses takes the whole batch down with it — the binder is never
	// left holding the prefix of a sweep the user has already put away.
	Describe("a card the database refuses", func() {
		var before []uuid.UUID

		BeforeEach(func() {
			seedSlot(binderID, cardID, 0)
			before = positionsOf(binderID)
			batch = []model.Slot{namedSlot(binderID, cardID, 1), namedSlot(binderID, cardID, 2), namedSlot(binderID, cardID, 3)}
		})

		Context("because its card is not in the card database", func() {
			BeforeEach(func() {
				batch[1].CardID = uuid.New()
			})

			It("reports the card as an invalid reference", func() {
				Expect(dataerror.IsInvalidReferenceError(err, model.ReferenceCard)).To(BeTrue(),
					"got %v", err)
			})

			It("writes none of the batch, not even the entries before the bad one", func() {
				Expect(positionsOf(binderID)).To(Equal(before))
			})
		})

		Context("because its printing is a printing of a different card", func() {
			BeforeEach(func() {
				otherPrinting := seedPrinting(seedCard("Blue-Eyes White Dragon"), "SDK-EN001", "Ultra Rare")
				batch[2].CardPrintingID = &otherPrinting
				batch[2].SetResolution = cardmodel.SetResolutionManual
			})

			It("reports the printing as an invalid reference", func() {
				Expect(dataerror.IsInvalidReferenceError(err, model.ReferenceCardPrinting)).To(BeTrue(),
					"got %v", err)
			})

			It("writes none of the batch", func() {
				Expect(positionsOf(binderID)).To(Equal(before))
			})
		})

		Context("because a position in it is already occupied", func() {
			BeforeEach(func() {
				batch[0].Position = 0
			})

			It("reports the conflict", func() {
				Expect(dataerror.IsConflictError(err)).To(BeTrue(), "got %v", err)
			})

			It("writes none of the batch", func() {
				Expect(positionsOf(binderID)).To(Equal(before))
			})
		})

		Context("because two entries of the batch claim one position", func() {
			BeforeEach(func() {
				batch[1].Position = batch[0].Position
			})

			It("reports the conflict rather than writing one of the two", func() {
				Expect(dataerror.IsConflictError(err)).To(BeTrue(), "got %v", err)
				Expect(positionsOf(binderID)).To(Equal(before))
			})
		})
	})
})

// A separate top-level Describe rather than a Context inside the one above: the
// ancestor's JustBeforeEach would insert the batch before this one's ran, and
// the second insert would collide on the positions the first took.
var _ = Describe("InsertSlots inside a transaction the caller opened", func() {
	var ctx = context.Background()

	// errLaterStep stands for any step of the caller's work that fails after
	// the batch is written. The specs assert what the binder holds, never this
	// error's text.
	var errLaterStep = errors.New("a later step of the caller's work failed")

	var (
		subject    *store.Store
		binderID   uuid.UUID
		cardID     uuid.UUID
		seenInside []model.Slot
		txErr      error
	)

	BeforeEach(func() {
		subject = store.NewStore(testDB)
		binderID = seedBinder(seedUser(), "Main binder")
		cardID = seedCard("Dark Magician")
	})

	AfterEach(truncateAll)

	// The batch is written inside whatever transaction the caller opened, not a
	// transaction of its own. That is what makes the service's InTx worth
	// anything: the ownership check, the count and the insert stand or fall
	// together, so a failure anywhere in the commit leaves the binder untouched
	// rather than holding the prefix of a sweep the user has already put away.
	JustBeforeEach(func() {
		txErr = subject.InTx(ctx, func(ctx context.Context) error {
			written, insertErr := subject.InsertSlots(ctx, []model.Slot{
				namedSlot(binderID, cardID, 0),
				namedSlot(binderID, cardID, 1),
				namedSlot(binderID, cardID, 2),
			})
			Expect(insertErr).NotTo(HaveOccurred())
			seenInside = written
			return errLaterStep
		})
	})

	It("had written every slot inside the transaction", func() {
		Expect(seenInside).To(HaveLen(3))
	})

	It("rolls the whole batch back when the caller fails after it", func() {
		Expect(txErr).To(MatchError(errLaterStep))
		Expect(positionsOf(binderID)).To(BeEmpty())
	})
})

// namedSlot is a slot resolved by name, so it carries no printing and needs
// nothing seeded beyond the card.
func namedSlot(binderID, cardID uuid.UUID, position int) model.Slot {
	return model.Slot{
		ID:             uuid.New(),
		BinderID:       binderID,
		Position:       position,
		CardID:         cardID,
		CardPrintingID: nil,
		SetResolution:  cardmodel.SetResolutionByName,
		// Zero on purpose: the timestamps come back from the database.
		CreatedAt: time.Time{},
		UpdatedAt: time.Time{},
	}
}

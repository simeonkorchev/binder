package store_test

import (
	"context"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/simeonkorchev/binder/internal/binder/store"
)

// Every spec here runs against the real UNIQUE (binder_id, position). That
// constraint is not deferrable, so a rearrangement that is correct only at the
// end of the statement fails: positionsOf then asserts density on the way out,
// which is the invariant the database cannot state for itself.
var _ = Describe("Position rearrangement", func() {
	var ctx = context.Background()

	var (
		subject  *store.Store
		binderID uuid.UUID
		slotIDs  []uuid.UUID
	)

	// seedFullBinder fills the binder with count slots at 0..count-1 and
	// returns their ids in position order.
	seedFullBinder := func(count int) []uuid.UUID {
		ids := make([]uuid.UUID, 0, count)
		for position := range count {
			ids = append(ids, seedSlot(binderID, seedCard("Card"), position))
		}
		return ids
	}

	BeforeEach(func() {
		subject = store.NewStore(testDB)
		binderID = seedBinder(seedUser(), "Main binder")
	})

	AfterEach(truncateAll)

	Describe("OpenPositionGap", func() {
		var err error

		BeforeEach(func() {
			slotIDs = seedFullBinder(5)
		})

		When("the gap is opened in the middle of a full binder", func() {
			JustBeforeEach(func() {
				err = subject.OpenPositionGap(ctx, binderID, 2)
			})

			// Every position from 2 to 4 is occupied, so shifting them one
			// along collides at every single step: 2 wants 3, which is taken by
			// a row that wants 4, which is taken.
			It("frees the position without ever holding two slots at one", func() {
				Expect(err).NotTo(HaveOccurred())

				var occupied []int
				Expect(testDB.SelectContext(ctx, &occupied,
					`SELECT "position" FROM binder_slots WHERE binder_id = $1 ORDER BY "position"`,
					binderID)).To(Succeed())
				Expect(occupied).To(Equal([]int{0, 1, 3, 4, 5}))
			})

			It("keeps the slots before the gap where they were, in order", func() {
				var ordered []uuid.UUID
				Expect(testDB.SelectContext(ctx, &ordered,
					`SELECT id FROM binder_slots WHERE binder_id = $1 ORDER BY "position"`,
					binderID)).To(Succeed())
				Expect(ordered).To(Equal([]uuid.UUID{
					slotIDs[0], slotIDs[1], slotIDs[2], slotIDs[3], slotIDs[4],
				}))
			})
		})

		When("the gap is opened at the very front", func() {
			JustBeforeEach(func() {
				err = subject.OpenPositionGap(ctx, binderID, 0)
			})

			It("shifts every slot along, colliding at every step", func() {
				Expect(err).NotTo(HaveOccurred())

				var occupied []int
				Expect(testDB.SelectContext(ctx, &occupied,
					`SELECT "position" FROM binder_slots WHERE binder_id = $1 ORDER BY "position"`,
					binderID)).To(Succeed())
				Expect(occupied).To(Equal([]int{1, 2, 3, 4, 5}))
			})
		})

		When("the gap is opened past the last slot", func() {
			JustBeforeEach(func() {
				err = subject.OpenPositionGap(ctx, binderID, 5)
			})

			It("changes nothing", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(positionsOf(binderID)).To(Equal(slotIDs))
			})
		})

		When("the binder is empty", func() {
			BeforeEach(func() {
				truncateSlots(binderID)
				slotIDs = nil
			})

			JustBeforeEach(func() {
				err = subject.OpenPositionGap(ctx, binderID, 0)
			})

			It("succeeds with nothing to shift", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(positionsOf(binderID)).To(BeEmpty())
			})
		})
	})

	Describe("ClosePositionGap", func() {
		var err error

		BeforeEach(func() {
			slotIDs = seedFullBinder(5)
		})

		When("the slot in the middle has been removed", func() {
			BeforeEach(func() {
				Expect(subject.DeleteSlotByID(ctx, binderID, slotIDs[2])).To(Succeed())
			})

			JustBeforeEach(func() {
				err = subject.ClosePositionGap(ctx, binderID, 2)
			})

			It("closes the hole and leaves the positions dense", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(positionsOf(binderID)).To(Equal([]uuid.UUID{
					slotIDs[0], slotIDs[1], slotIDs[3], slotIDs[4],
				}))
			})
		})

		When("the first slot has been removed", func() {
			BeforeEach(func() {
				Expect(subject.DeleteSlotByID(ctx, binderID, slotIDs[0])).To(Succeed())
			})

			JustBeforeEach(func() {
				err = subject.ClosePositionGap(ctx, binderID, 0)
			})

			It("pulls the whole binder back one place", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(positionsOf(binderID)).To(Equal([]uuid.UUID{
					slotIDs[1], slotIDs[2], slotIDs[3], slotIDs[4],
				}))
			})
		})

		When("the last slot has been removed", func() {
			BeforeEach(func() {
				Expect(subject.DeleteSlotByID(ctx, binderID, slotIDs[4])).To(Succeed())
			})

			JustBeforeEach(func() {
				err = subject.ClosePositionGap(ctx, binderID, 4)
			})

			It("leaves the rest alone: there is no hole to close", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(positionsOf(binderID)).To(Equal(slotIDs[:4]))
			})
		})
	})

	Describe("MoveSlotPosition", func() {
		var err error

		BeforeEach(func() {
			slotIDs = seedFullBinder(5)
		})

		When("a slot moves later in the binder", func() {
			JustBeforeEach(func() {
				err = subject.MoveSlotPosition(ctx, binderID, slotIDs[1], 1, 3)
			})

			It("puts it at its destination and closes up behind it", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(positionsOf(binderID)).To(Equal([]uuid.UUID{
					slotIDs[0], slotIDs[2], slotIDs[3], slotIDs[1], slotIDs[4],
				}))
			})
		})

		When("a slot moves earlier in the binder", func() {
			JustBeforeEach(func() {
				err = subject.MoveSlotPosition(ctx, binderID, slotIDs[3], 3, 1)
			})

			It("puts it at its destination and pushes the others along", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(positionsOf(binderID)).To(Equal([]uuid.UUID{
					slotIDs[0], slotIDs[3], slotIDs[1], slotIDs[2], slotIDs[4],
				}))
			})
		})

		// The whole binder is the affected window here, so every position in it
		// is occupied at every intermediate step. This is the case a per-row
		// shift cannot do at all.
		When("the last slot moves to the front of a full binder", func() {
			JustBeforeEach(func() {
				err = subject.MoveSlotPosition(ctx, binderID, slotIDs[4], 4, 0)
			})

			It("rotates the whole binder without tripping the unique constraint", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(positionsOf(binderID)).To(Equal([]uuid.UUID{
					slotIDs[4], slotIDs[0], slotIDs[1], slotIDs[2], slotIDs[3],
				}))
			})
		})

		When("the first slot moves to the back of a full binder", func() {
			JustBeforeEach(func() {
				err = subject.MoveSlotPosition(ctx, binderID, slotIDs[0], 0, 4)
			})

			It("rotates the whole binder the other way", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(positionsOf(binderID)).To(Equal([]uuid.UUID{
					slotIDs[1], slotIDs[2], slotIDs[3], slotIDs[4], slotIDs[0],
				}))
			})
		})

		When("a slot moves across a page boundary", func() {
			BeforeEach(func() {
				truncateSlots(binderID)
				slotIDs = seedFullBinder(12)
			})

			JustBeforeEach(func() {
				err = subject.MoveSlotPosition(ctx, binderID, slotIDs[10], 10, 2)
			})

			It("rearranges both pages in one statement", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(positionsOf(binderID)).To(Equal([]uuid.UUID{
					slotIDs[0], slotIDs[1], slotIDs[10],
					slotIDs[2], slotIDs[3], slotIDs[4], slotIDs[5], slotIDs[6], slotIDs[7],
					slotIDs[8], slotIDs[9], slotIDs[11],
				}))
			})
		})

		When("a slot is moved to the position it already holds", func() {
			JustBeforeEach(func() {
				err = subject.MoveSlotPosition(ctx, binderID, slotIDs[2], 2, 2)
			})

			It("leaves the binder as it was", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(positionsOf(binderID)).To(Equal(slotIDs))
			})
		})

		When("another binder holds slots at the same positions", func() {
			var otherBinderID uuid.UUID
			var otherSlotIDs []uuid.UUID

			BeforeEach(func() {
				otherBinderID = seedBinder(seedUser(), "Someone else's binder")
				otherSlotIDs = []uuid.UUID{
					seedSlot(otherBinderID, seedCard("Other card"), 0),
					seedSlot(otherBinderID, seedCard("Other card"), 1),
				}
			})

			JustBeforeEach(func() {
				err = subject.MoveSlotPosition(ctx, binderID, slotIDs[4], 4, 0)
			})

			It("does not touch them", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(positionsOf(otherBinderID)).To(Equal(otherSlotIDs))
			})
		})
	})
})

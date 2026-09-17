package api_test

import (
	"net/http"
	"net/http/httptest"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/simeonkorchev/binder/internal/binder/api"
	"github.com/simeonkorchev/binder/internal/binder/api/apifakes"
	"github.com/simeonkorchev/binder/internal/binder/model"
	"github.com/simeonkorchev/binder/internal/binder/service"
	cardmodel "github.com/simeonkorchev/binder/internal/card/model"
)

var _ = Describe("POST /binders/{binderId}/slots/batch", func() {
	var (
		ownerID    = uuid.New()
		binderID   = uuid.New()
		cardID     = uuid.New()
		printingID = uuid.New()
		batchPath  = "/binders/" + binderID.String() + "/slots/batch"
	)

	var (
		fakeSvc *apifakes.FakeService
		actor   api.ActorFunc
		cards   []map[string]any
		body    map[string]any
		rec     *httptest.ResponseRecorder
	)

	BeforeEach(func() {
		fakeSvc = new(apifakes.FakeService)
		actor = signedInAs(ownerID)

		cards = []map[string]any{
			{"cardId": cardID.String(), "cardPrintingId": printingID.String(), "setResolution": "manual"},
			{"cardId": cardID.String(), "setResolution": "by_name"},
		}
		fakeSvc.AddSlotsReturns([]model.Slot{
			{
				ID: uuid.New(), BinderID: binderID, Position: 4, CardID: cardID,
				CardPrintingID: &printingID, SetResolution: cardmodel.SetResolutionManual,
			},
			{
				ID: uuid.New(), BinderID: binderID, Position: 5, CardID: cardID,
				CardPrintingID: nil, SetResolution: cardmodel.SetResolutionByName,
			},
		}, nil)
	})

	JustBeforeEach(func() {
		body = map[string]any{"cards": cards}
		rec = do(fakeSvc, actor, http.MethodPost, batchPath, body)
	})

	When("the session is committed", func() {
		It("answers 201 with the slots, in the order they landed", func() {
			Expect(rec.Code).To(Equal(http.StatusCreated))

			var got struct {
				Slots []struct {
					Position       int        `json:"position"`
					Page           int        `json:"page"`
					SlotOnPage     int        `json:"slotOnPage"`
					CardID         uuid.UUID  `json:"cardId"`
					CardPrintingID *uuid.UUID `json:"cardPrintingId"`
					SetResolution  string     `json:"setResolution"`
				} `json:"slots"`
			}
			decodeBody(rec, &got)

			Expect(got.Slots).To(HaveLen(2))
			Expect(got.Slots[0].Position).To(Equal(4))
			Expect(got.Slots[0].Page).To(BeZero())
			Expect(got.Slots[0].SlotOnPage).To(Equal(4))
			Expect(got.Slots[0].CardID).To(Equal(cardID))
			Expect(got.Slots[0].CardPrintingID).To(HaveValue(Equal(printingID)))
			Expect(got.Slots[0].SetResolution).To(Equal("manual"))
			Expect(got.Slots[1].Position).To(Equal(5))
			Expect(got.Slots[1].CardPrintingID).To(BeNil())
		})

		It("passes the actor, the binder from the path and the cards in order", func() {
			Expect(fakeSvc.AddSlotsCallCount()).To(Equal(1))

			_, passedOwner, passedBinder, input := fakeSvc.AddSlotsArgsForCall(0)
			Expect(passedOwner).To(Equal(ownerID))
			Expect(passedBinder).To(Equal(binderID))
			Expect(input.Cards).To(Equal([]model.SlotCard{
				{CardID: cardID, CardPrintingID: &printingID, SetResolution: cardmodel.SetResolutionManual},
				{CardID: cardID, CardPrintingID: nil, SetResolution: cardmodel.SetResolutionByName},
			}))
		})
	})

	// 000-principles.md section 8b: a commit of nothing is a commit, and its
	// slots serialise as [] rather than null.
	When("the batch is empty", func() {
		BeforeEach(func() {
			cards = []map[string]any{}
			fakeSvc.AddSlotsReturns([]model.Slot{}, nil)
		})

		It("answers 201 with no slots, and still calls the service", func() {
			Expect(rec.Code).To(Equal(http.StatusCreated))
			Expect(rec.Body.String()).To(ContainSubstring(`"slots":[]`))
			Expect(fakeSvc.AddSlotsCallCount()).To(Equal(1))
		})
	})

	// The boundary guard (000-principles.md section 10). Each of these is a 422
	// naming the entry that is wrong, and the service is never reached.
	Describe("an entry whose resolution and printing disagree", func() {
		AfterEach(func() {
			Expect(rec.Code).To(Equal(http.StatusUnprocessableEntity))
			Expect(fakeSvc.AddSlotsCallCount()).To(BeZero())
		})

		When("a resolution that determines a set arrives without its printing", func() {
			BeforeEach(func() {
				cards[0] = map[string]any{"cardId": cardID.String(), "setResolution": "exact"}
			})

			It("says which entry and which field", func() {
				Expect(rec.Body.String()).To(ContainSubstring("body.cards[0].cardPrintingId"))
			})
		})

		When("a resolution that determines no set arrives carrying a printing", func() {
			BeforeEach(func() {
				cards[1] = map[string]any{
					"cardId": cardID.String(), "cardPrintingId": printingID.String(), "setResolution": "by_name",
				}
			})

			It("points at the second entry, not the first", func() {
				Expect(rec.Body.String()).To(ContainSubstring("body.cards[1].cardPrintingId"))
			})
		})

		When("several entries are wrong", func() {
			BeforeEach(func() {
				cards[0] = map[string]any{"cardId": cardID.String(), "setResolution": "exact"}
				cards[1] = map[string]any{
					"cardId": cardID.String(), "cardPrintingId": printingID.String(), "setResolution": "unresolved",
				}
			})

			It("reports all of them, so a sixty-card commit is fixed once", func() {
				Expect(rec.Body.String()).To(ContainSubstring("body.cards[0].cardPrintingId"))
				Expect(rec.Body.String()).To(ContainSubstring("body.cards[1].cardPrintingId"))
			})
		})

		When("a resolution is not one a slot can record", func() {
			BeforeEach(func() {
				cards[0] = map[string]any{"cardId": cardID.String(), "setResolution": "probably"}
			})

			It("is refused by the enum in the schema", func() {
				Expect(rec.Body.String()).To(ContainSubstring("body.cards[0].setResolution"))
			})
		})
	})

	// The first half of the bound. huma refuses an over-long batch from the
	// maxItems in the schema, before a handler runs; the service refuses it
	// again, which is the specs in internal/binder/service.
	When("the batch is longer than the bound allows", func() {
		BeforeEach(func() {
			cards = make([]map[string]any, 0, model.MaxSlotsPerBatch+1)
			for range model.MaxSlotsPerBatch + 1 {
				cards = append(cards, map[string]any{"cardId": cardID.String(), "setResolution": "by_name"})
			}
		})

		It("answers 422 and never reaches the service", func() {
			Expect(rec.Code).To(Equal(http.StatusUnprocessableEntity))
			Expect(fakeSvc.AddSlotsCallCount()).To(BeZero())
		})
	})

	When("the service says the batch is too large", func() {
		BeforeEach(func() {
			fakeSvc.AddSlotsReturns(nil, service.ErrBatchTooLarge)
		})

		It("answers 422 from the table", func() {
			Expect(rec.Code).To(Equal(http.StatusUnprocessableEntity))
		})
	})

	// 404 and not 403: a distinct "forbidden" would confirm to a stranger that
	// the binder exists.
	When("the binder is not the actor's", func() {
		BeforeEach(func() {
			fakeSvc.AddSlotsReturns(nil, service.ErrBinderNotFound)
		})

		It("answers 404", func() {
			Expect(rec.Code).To(Equal(http.StatusNotFound))
		})
	})

	When("a concurrent change took a position", func() {
		BeforeEach(func() {
			fakeSvc.AddSlotsReturns(nil, service.ErrPositionTaken)
		})

		It("answers 409, which is a retry rather than a correction", func() {
			Expect(rec.Code).To(Equal(http.StatusConflict))
		})
	})

	When("nobody is signed in", func() {
		BeforeEach(func() {
			actor = signedOut()
		})

		It("answers 401 and never reaches the service", func() {
			Expect(rec.Code).To(Equal(http.StatusUnauthorized))
			Expect(fakeSvc.AddSlotsCallCount()).To(BeZero())
		})
	})
})

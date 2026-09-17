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

var _ = Describe("Binder slot endpoints", func() {
	var (
		ownerID    = uuid.New()
		binderID   = uuid.New()
		slotID     = uuid.New()
		cardID     = uuid.New()
		printingID = uuid.New()
		slotsPath  = "/binders/" + binderID.String() + "/slots"
	)

	var (
		fakeSvc *apifakes.FakeService
		actor   api.ActorFunc
		body    map[string]any
		rec     *httptest.ResponseRecorder
	)

	BeforeEach(func() {
		fakeSvc = new(apifakes.FakeService)
		actor = signedInAs(ownerID)
	})

	Describe("POST /binders/{binderId}/slots", func() {
		BeforeEach(func() {
			body = map[string]any{
				"cardId":         cardID.String(),
				"cardPrintingId": printingID.String(),
				"setResolution":  "exact",
			}
			fakeSvc.AddSlotReturns(model.Slot{
				ID:             slotID,
				BinderID:       binderID,
				Position:       4,
				CardID:         cardID,
				CardPrintingID: &printingID,
				SetResolution:  cardmodel.SetResolutionExact,
			}, nil)
		})

		JustBeforeEach(func() {
			rec = do(fakeSvc, actor, http.MethodPost, slotsPath, body)
		})

		When("the card is added", func() {
			It("answers 201 with the slot and where it landed", func() {
				Expect(rec.Code).To(Equal(http.StatusCreated))

				var got struct {
					ID             uuid.UUID  `json:"id"`
					Position       int        `json:"position"`
					Page           int        `json:"page"`
					SlotOnPage     int        `json:"slotOnPage"`
					CardID         uuid.UUID  `json:"cardId"`
					CardPrintingID *uuid.UUID `json:"cardPrintingId"`
					SetResolution  string     `json:"setResolution"`
				}
				decodeBody(rec, &got)

				Expect(got.ID).To(Equal(slotID))
				Expect(got.Position).To(Equal(4))
				Expect(got.Page).To(BeZero())
				Expect(got.SlotOnPage).To(Equal(4))
				Expect(got.CardID).To(Equal(cardID))
				Expect(got.CardPrintingID).To(HaveValue(Equal(printingID)))
				Expect(got.SetResolution).To(Equal("exact"))
			})

			It("passes the actor, the binder from the path and the card, with no position", func() {
				Expect(fakeSvc.AddSlotCallCount()).To(Equal(1))
				_, passedOwnerID, passedBinderID, input := fakeSvc.AddSlotArgsForCall(0)
				Expect(passedOwnerID).To(Equal(ownerID))
				Expect(passedBinderID).To(Equal(binderID))
				Expect(input).To(Equal(model.AddSlotInput{
					SlotCard: model.SlotCard{
						CardID:         cardID,
						CardPrintingID: &printingID,
						SetResolution:  cardmodel.SetResolutionExact,
					},
					Position: nil,
				}))
			})
		})

		When("a position is given", func() {
			BeforeEach(func() {
				body["position"] = 2
			})

			It("passes it through", func() {
				_, _, _, input := fakeSvc.AddSlotArgsForCall(0)
				Expect(input.Position).To(HaveValue(Equal(2)))
			})
		})

		// The boundary half of 000-principles.md section 10: the pairing the
		// binder_slots CHECK states is refused here, naming the field, so the
		// client is told what is wrong rather than getting a bare 422.
		When("a code-rung resolution arrives without its printing", func() {
			BeforeEach(func() {
				delete(body, "cardPrintingId")
			})

			It("is refused at the boundary, naming the field", func() {
				Expect(rec.Code).To(Equal(http.StatusUnprocessableEntity))
				Expect(rec.Body.String()).To(ContainSubstring("body.cardPrintingId"))
				Expect(fakeSvc.AddSlotCallCount()).To(BeZero())
			})
		})

		When("a name-rung resolution arrives carrying a printing", func() {
			BeforeEach(func() {
				body["setResolution"] = "by_name"
			})

			It("is refused at the boundary, naming the field", func() {
				Expect(rec.Code).To(Equal(http.StatusUnprocessableEntity))
				Expect(rec.Body.String()).To(ContainSubstring("body.cardPrintingId"))
				Expect(fakeSvc.AddSlotCallCount()).To(BeZero())
			})
		})

		When("the resolution is not a rung of the ladder", func() {
			BeforeEach(func() {
				body["setResolution"] = "probably"
			})

			It("is refused by the schema's enum", func() {
				Expect(rec.Code).To(Equal(http.StatusUnprocessableEntity))
				Expect(fakeSvc.AddSlotCallCount()).To(BeZero())
			})
		})

		When("the service refuses the position", func() {
			BeforeEach(func() {
				fakeSvc.AddSlotReturns(model.Slot{}, service.ErrPositionOutOfRange)
			})

			It("answers 422 from the table", func() {
				Expect(rec.Code).To(Equal(http.StatusUnprocessableEntity))
				Expect(rec.Body.String()).To(ContainSubstring("That position is outside the binder."))
			})
		})

		When("the card is not in the card database", func() {
			BeforeEach(func() {
				fakeSvc.AddSlotReturns(model.Slot{}, service.ErrCardUnknown)
			})

			It("answers 422 from the table", func() {
				Expect(rec.Code).To(Equal(http.StatusUnprocessableEntity))
				Expect(rec.Body.String()).To(ContainSubstring("That card is not in the card database."))
			})
		})

		When("a concurrent change took the position", func() {
			BeforeEach(func() {
				fakeSvc.AddSlotReturns(model.Slot{}, service.ErrPositionTaken)
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
				Expect(fakeSvc.AddSlotCallCount()).To(BeZero())
			})
		})
	})

	Describe("PATCH /binders/{binderId}/slots", func() {
		BeforeEach(func() {
			body = map[string]any{"slotId": slotID.String(), "toPosition": 3}
		})

		JustBeforeEach(func() {
			rec = do(fakeSvc, actor, http.MethodPatch, slotsPath, body)
		})

		When("the move succeeds", func() {
			It("answers 204 with no body", func() {
				Expect(rec.Code).To(Equal(http.StatusNoContent))
				Expect(rec.Body.String()).To(BeEmpty())
			})

			It("passes the actor, the binder and the move", func() {
				Expect(fakeSvc.MoveSlotCallCount()).To(Equal(1))
				_, passedOwnerID, passedBinderID, input := fakeSvc.MoveSlotArgsForCall(0)
				Expect(passedOwnerID).To(Equal(ownerID))
				Expect(passedBinderID).To(Equal(binderID))
				Expect(input).To(Equal(model.MoveSlotInput{SlotID: slotID, ToPosition: 3}))
			})
		})

		When("the destination is negative", func() {
			BeforeEach(func() {
				body["toPosition"] = -1
			})

			It("is refused at the boundary, without reaching the service", func() {
				Expect(rec.Code).To(Equal(http.StatusUnprocessableEntity))
				Expect(fakeSvc.MoveSlotCallCount()).To(BeZero())
			})
		})

		When("the slot is not in that binder", func() {
			BeforeEach(func() {
				fakeSvc.MoveSlotReturns(service.ErrSlotNotFound)
			})

			It("answers 404 from the table", func() {
				Expect(rec.Code).To(Equal(http.StatusNotFound))
				Expect(rec.Body.String()).To(ContainSubstring("That card is not in this binder."))
			})
		})

		When("nobody is signed in", func() {
			BeforeEach(func() {
				actor = signedOut()
			})

			It("answers 401 and never reaches the service", func() {
				Expect(rec.Code).To(Equal(http.StatusUnauthorized))
				Expect(fakeSvc.MoveSlotCallCount()).To(BeZero())
			})
		})
	})

	Describe("DELETE /binders/{binderId}/slots/{slotId}", func() {
		var target string

		BeforeEach(func() {
			target = slotsPath + "/" + slotID.String()
		})

		JustBeforeEach(func() {
			rec = do(fakeSvc, actor, http.MethodDelete, target, nil)
		})

		When("the card is removed", func() {
			It("answers 204 with no body", func() {
				Expect(rec.Code).To(Equal(http.StatusNoContent))
				Expect(rec.Body.String()).To(BeEmpty())
			})

			It("passes the actor, the binder and the slot from the path", func() {
				Expect(fakeSvc.RemoveSlotCallCount()).To(Equal(1))
				_, passedOwnerID, passedBinderID, passedSlotID := fakeSvc.RemoveSlotArgsForCall(0)
				Expect(passedOwnerID).To(Equal(ownerID))
				Expect(passedBinderID).To(Equal(binderID))
				Expect(passedSlotID).To(Equal(slotID))
			})
		})

		When("the binder is somebody else's", func() {
			BeforeEach(func() {
				fakeSvc.RemoveSlotReturns(service.ErrBinderNotFound)
			})

			It("answers 404, which tells a stranger nothing", func() {
				Expect(rec.Code).To(Equal(http.StatusNotFound))
			})
		})

		When("nobody is signed in", func() {
			BeforeEach(func() {
				actor = signedOut()
			})

			It("answers 401 and never reaches the service", func() {
				Expect(rec.Code).To(Equal(http.StatusUnauthorized))
				Expect(fakeSvc.RemoveSlotCallCount()).To(BeZero())
			})
		})

		When("the service fails for a reason the table does not name", func() {
			BeforeEach(func() {
				fakeSvc.RemoveSlotReturns(errService)
			})

			It("answers an opaque 500", func() {
				Expect(rec.Code).To(Equal(http.StatusInternalServerError))
				Expect(rec.Body.String()).NotTo(ContainSubstring("service failed"))
			})
		})
	})
})

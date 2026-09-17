package api_test

import (
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	cardmodel "github.com/simeonkorchev/binder/internal/card/model"
	"github.com/simeonkorchev/binder/internal/listing/api"
	"github.com/simeonkorchev/binder/internal/listing/api/apifakes"
	"github.com/simeonkorchev/binder/internal/listing/model"
	"github.com/simeonkorchev/binder/internal/listing/service"
)

var _ = Describe("Listing endpoints", func() {
	var (
		sellerID  = uuid.New()
		slotID    = uuid.New()
		listingID = uuid.New()
		listedAt  = time.Date(2026, time.September, 1, 9, 0, 0, 0, time.UTC)
	)

	var (
		fakeSvc *apifakes.FakeService
		actor   api.ActorFunc
		body    map[string]any
		rec     *httptest.ResponseRecorder
	)

	BeforeEach(func() {
		fakeSvc = new(apifakes.FakeService)
		actor = signedInAs(sellerID)
	})

	Describe("POST /listings", func() {
		BeforeEach(func() {
			body = map[string]any{"binderSlotId": slotID.String()}
			fakeSvc.CreateListingReturns(model.Listing{
				ID:           listingID,
				BinderSlotID: slotID,
				CreatedAt:    listedAt,
				UpdatedAt:    listedAt,
			}, nil)
		})

		JustBeforeEach(func() {
			rec = do(fakeSvc, actor, http.MethodPost, "/listings", body)
		})

		When("the card goes up for sale", func() {
			It("answers 201 with the listing", func() {
				Expect(rec.Code).To(Equal(http.StatusCreated))

				var got struct {
					ID           uuid.UUID `json:"id"`
					BinderSlotID uuid.UUID `json:"binderSlotId"`
					CreatedAt    time.Time `json:"createdAt"`
					UpdatedAt    time.Time `json:"updatedAt"`
				}
				decodeBody(rec, &got)

				Expect(got.ID).To(Equal(listingID))
				Expect(got.BinderSlotID).To(Equal(slotID))
				Expect(got.CreatedAt).To(BeTemporally("==", listedAt))
				Expect(got.UpdatedAt).To(BeTemporally("==", listedAt))
			})

			It("passes the actor and the slot from the body", func() {
				Expect(fakeSvc.CreateListingCallCount()).To(Equal(1))
				_, passedOwnerID, passedSlotID := fakeSvc.CreateListingArgsForCall(0)
				Expect(passedOwnerID).To(Equal(sellerID))
				Expect(passedSlotID).To(Equal(slotID))
			})
		})

		// The boundary half of 000-principles.md section 10: the nil uuid is
		// representable but names no slot that can exist, so the client is told
		// which field is wrong instead of being answered "not found".
		When("the slot id is the nil uuid", func() {
			BeforeEach(func() {
				body["binderSlotId"] = uuid.Nil.String()
			})

			It("is refused at the boundary, naming the field, and the service is never reached", func() {
				Expect(rec.Code).To(Equal(http.StatusUnprocessableEntity))
				Expect(rec.Body.String()).To(ContainSubstring("body.binderSlotId"))
				Expect(fakeSvc.CreateListingCallCount()).To(BeZero())
			})
		})

		When("the slot id is missing", func() {
			BeforeEach(func() {
				body = map[string]any{}
			})

			It("is refused, and the service is never reached", func() {
				Expect(rec.Code).To(Equal(http.StatusUnprocessableEntity))
				Expect(fakeSvc.CreateListingCallCount()).To(BeZero())
			})
		})

		When("the card is not in one of the actor's binders", func() {
			BeforeEach(func() {
				fakeSvc.CreateListingReturns(model.Listing{}, service.ErrBinderSlotNotFound)
			})

			It("answers 404, which is also the answer for somebody else's card", func() {
				Expect(rec.Code).To(Equal(http.StatusNotFound))
			})
		})

		When("the card is already for sale", func() {
			BeforeEach(func() {
				fakeSvc.CreateListingReturns(model.Listing{}, service.ErrAlreadyListed)
			})

			It("answers 409", func() {
				Expect(rec.Code).To(Equal(http.StatusConflict))
			})
		})

		When("the service fails for an unforeseen reason", func() {
			BeforeEach(func() {
				fakeSvc.CreateListingReturns(model.Listing{}, errService)
			})

			It("answers 500 without the detail", func() {
				Expect(rec.Code).To(Equal(http.StatusInternalServerError))
				Expect(rec.Body.String()).NotTo(ContainSubstring(errService.Error()))
			})
		})

		When("nobody is signed in", func() {
			BeforeEach(func() {
				actor = signedOut()
			})

			It("answers 401 and never reaches the service", func() {
				Expect(rec.Code).To(Equal(http.StatusUnauthorized))
				Expect(fakeSvc.CreateListingCallCount()).To(BeZero())
			})
		})
	})

	Describe("DELETE /listings/{listingId}", func() {
		JustBeforeEach(func() {
			rec = do(fakeSvc, actor, http.MethodDelete, "/listings/"+listingID.String(), nil)
		})

		When("the listing comes down", func() {
			It("answers 204 with no body, passing the actor and the listing from the path", func() {
				Expect(rec.Code).To(Equal(http.StatusNoContent))
				Expect(rec.Body.Len()).To(BeZero())

				Expect(fakeSvc.DeleteListingCallCount()).To(Equal(1))
				_, passedOwnerID, passedListingID := fakeSvc.DeleteListingArgsForCall(0)
				Expect(passedOwnerID).To(Equal(sellerID))
				Expect(passedListingID).To(Equal(listingID))
			})
		})

		When("the listing is not the actor's", func() {
			BeforeEach(func() {
				fakeSvc.DeleteListingReturns(service.ErrListingNotFound)
			})

			It("answers 404 rather than 403, so a stranger learns nothing", func() {
				Expect(rec.Code).To(Equal(http.StatusNotFound))
			})
		})

		When("nobody is signed in", func() {
			BeforeEach(func() {
				actor = signedOut()
			})

			It("answers 401 and never reaches the service", func() {
				Expect(rec.Code).To(Equal(http.StatusUnauthorized))
				Expect(fakeSvc.DeleteListingCallCount()).To(BeZero())
			})
		})
	})

	Describe("GET /listings", func() {
		var target string

		BeforeEach(func() {
			target = "/listings"
			objectKey := "cards/blue-eyes.jpg"
			setCode := "LOB-EN001"
			fakeSvc.BrowseListingsReturns([]model.ListedCard{{
				ListingID: listingID,
				SellerID:  sellerID,
				Card: cardmodel.Card{
					ID:             uuid.New(),
					Name:           "Blue-Eyes White Dragon",
					ImageObjectKey: &objectKey,
				},
				SetCode:  &setCode,
				ListedAt: listedAt,
			}}, nil)
		})

		JustBeforeEach(func() {
			rec = do(fakeSvc, actor, http.MethodGet, target, nil)
		})

		When("cards are for sale", func() {
			It("answers 200 with what is for sale and who to ask", func() {
				Expect(rec.Code).To(Equal(http.StatusOK))

				var got struct {
					Listings []struct {
						ListingID      uuid.UUID `json:"listingId"`
						SellerID       uuid.UUID `json:"sellerId"`
						CardID         uuid.UUID `json:"cardId"`
						CardName       string    `json:"cardName"`
						ImageObjectKey *string   `json:"imageObjectKey"`
						SetCode        *string   `json:"setCode"`
						ListedAt       time.Time `json:"listedAt"`
					} `json:"listings"`
				}
				decodeBody(rec, &got)

				Expect(got.Listings).To(HaveLen(1))
				Expect(got.Listings[0].ListingID).To(Equal(listingID))
				Expect(got.Listings[0].SellerID).To(Equal(sellerID))
				Expect(got.Listings[0].CardName).To(Equal("Blue-Eyes White Dragon"))
				Expect(got.Listings[0].ImageObjectKey).To(HaveValue(Equal("cards/blue-eyes.jpg")))
				Expect(got.Listings[0].SetCode).To(HaveValue(Equal("LOB-EN001")))
				Expect(got.Listings[0].ListedAt).To(BeTemporally("==", listedAt))
			})

			// D4 gives a buyer a contact button, not a feed full of addresses:
			// the details are a separate, deliberate request for one seller.
			It("does not leak the seller's contact details into the feed", func() {
				Expect(rec.Body.String()).NotTo(ContainSubstring("email"))
				Expect(rec.Body.String()).NotTo(ContainSubstring("phone"))
			})

			It("asks for the newest listings, capped at what one browse returns", func() {
				_, search := fakeSvc.BrowseListingsArgsForCall(0)
				Expect(search.Limit).To(BeNumerically(">", 0))
				Expect(search.Query).To(BeEmpty())
				Expect(search.SetPrefix).To(BeEmpty())
			})
		})

		When("the buyer filters by name and set", func() {
			BeforeEach(func() {
				target = "/listings?q=dragon&set=LOB"
			})

			It("passes both filters through", func() {
				Expect(fakeSvc.BrowseListingsCallCount()).To(Equal(1))
				_, search := fakeSvc.BrowseListingsArgsForCall(0)
				Expect(search.Query).To(Equal("dragon"))
				Expect(search.SetPrefix).To(Equal("LOB"))
			})
		})

		When("nothing is for sale", func() {
			BeforeEach(func() {
				fakeSvc.BrowseListingsReturns([]model.ListedCard{}, nil)
			})

			It("answers 200 with an empty array, never null", func() {
				Expect(rec.Code).To(Equal(http.StatusOK))
				Expect(rec.Body.String()).To(ContainSubstring(`"listings":[]`))
			})
		})

		// The feed is the same for everybody and carries nothing personal, so a
		// buyer can look before signing in.
		When("nobody is signed in", func() {
			BeforeEach(func() {
				actor = signedOut()
			})

			It("still answers 200", func() {
				Expect(rec.Code).To(Equal(http.StatusOK))
				Expect(fakeSvc.BrowseListingsCallCount()).To(Equal(1))
			})
		})

		When("the feed cannot be read", func() {
			BeforeEach(func() {
				fakeSvc.BrowseListingsReturns(nil, errService)
			})

			It("answers 500 without the detail", func() {
				Expect(rec.Code).To(Equal(http.StatusInternalServerError))
				Expect(rec.Body.String()).NotTo(ContainSubstring(errService.Error()))
			})
		})
	})
})

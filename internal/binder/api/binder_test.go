package api_test

import (
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/simeonkorchev/binder/internal/binder/api"
	"github.com/simeonkorchev/binder/internal/binder/api/apifakes"
	"github.com/simeonkorchev/binder/internal/binder/model"
	"github.com/simeonkorchev/binder/internal/binder/service"
	cardmodel "github.com/simeonkorchev/binder/internal/card/model"
)

var _ = Describe("Binder endpoints", func() {
	var (
		ownerID  = uuid.New()
		binderID = uuid.New()
		binder   = model.Binder{
			ID:        binderID,
			OwnerID:   ownerID,
			Name:      "Main binder",
			CreatedAt: time.Date(2026, time.March, 1, 10, 0, 0, 0, time.UTC),
			UpdatedAt: time.Date(2026, time.March, 2, 10, 0, 0, 0, time.UTC),
		}
	)

	var (
		fakeSvc *apifakes.FakeService
		actor   api.ActorFunc
		rec     *httptest.ResponseRecorder
	)

	BeforeEach(func() {
		fakeSvc = new(apifakes.FakeService)
		actor = signedInAs(ownerID)
	})

	Describe("POST /binders", func() {
		var body map[string]any

		BeforeEach(func() {
			body = map[string]any{"name": "Main binder"}
		})

		JustBeforeEach(func() {
			rec = do(fakeSvc, actor, http.MethodPost, "/binders", body)
		})

		When("the service creates the binder", func() {
			BeforeEach(func() {
				fakeSvc.CreateBinderReturns(binder, nil)
			})

			It("answers 201 with the binder", func() {
				Expect(rec.Code).To(Equal(http.StatusCreated))

				var got struct {
					ID        uuid.UUID `json:"id"`
					Name      string    `json:"name"`
					CreatedAt time.Time `json:"createdAt"`
				}
				decodeBody(rec, &got)
				Expect(got.ID).To(Equal(binderID))
				Expect(got.Name).To(Equal("Main binder"))
				Expect(got.CreatedAt).To(BeTemporally("==", binder.CreatedAt))
			})

			It("takes the owner from the session, not the request", func() {
				Expect(fakeSvc.CreateBinderCallCount()).To(Equal(1))
				_, passedOwnerID, name := fakeSvc.CreateBinderArgsForCall(0)
				Expect(passedOwnerID).To(Equal(ownerID))
				Expect(name).To(Equal("Main binder"))
			})

			It("does not tell the client who owns the binder", func() {
				Expect(rec.Body.String()).NotTo(ContainSubstring(ownerID.String()))
			})
		})

		When("nobody is signed in", func() {
			BeforeEach(func() {
				actor = signedOut()
			})

			It("answers 401 and never reaches the service", func() {
				Expect(rec.Code).To(Equal(http.StatusUnauthorized))
				Expect(fakeSvc.CreateBinderCallCount()).To(BeZero())
			})
		})

		When("the name is empty", func() {
			BeforeEach(func() {
				body["name"] = ""
			})

			It("is refused at the boundary, without reaching the service", func() {
				Expect(rec.Code).To(Equal(http.StatusUnprocessableEntity))
				Expect(fakeSvc.CreateBinderCallCount()).To(BeZero())
			})
		})
	})

	Describe("GET /binders", func() {
		JustBeforeEach(func() {
			rec = do(fakeSvc, actor, http.MethodGet, "/binders", nil)
		})

		When("the user has binders", func() {
			BeforeEach(func() {
				fakeSvc.ListBindersReturns([]model.Binder{binder}, nil)
			})

			It("answers 200 with them", func() {
				Expect(rec.Code).To(Equal(http.StatusOK))

				var got struct {
					Binders []struct {
						ID   uuid.UUID `json:"id"`
						Name string    `json:"name"`
					} `json:"binders"`
				}
				decodeBody(rec, &got)
				Expect(got.Binders).To(HaveLen(1))
				Expect(got.Binders[0].ID).To(Equal(binderID))
				Expect(got.Binders[0].Name).To(Equal("Main binder"))
			})

			It("asks for this user's binders", func() {
				_, passedOwnerID := fakeSvc.ListBindersArgsForCall(0)
				Expect(passedOwnerID).To(Equal(ownerID))
			})
		})

		When("the user has no binders yet", func() {
			BeforeEach(func() {
				fakeSvc.ListBindersReturns(nil, nil)
			})

			It("answers 200 with an empty array, not null and not an error", func() {
				Expect(rec.Code).To(Equal(http.StatusOK))
				Expect(rec.Body.String()).To(ContainSubstring(`"binders":[]`))
			})
		})

		When("nobody is signed in", func() {
			BeforeEach(func() {
				actor = signedOut()
			})

			It("answers 401 and never reaches the service", func() {
				Expect(rec.Code).To(Equal(http.StatusUnauthorized))
				Expect(fakeSvc.ListBindersCallCount()).To(BeZero())
			})
		})
	})

	Describe("PATCH /binders/{binderId}", func() {
		JustBeforeEach(func() {
			rec = do(fakeSvc, actor, http.MethodPatch, "/binders/"+binderID.String(),
				map[string]any{"name": "Trade binder"})
		})

		When("the binder is the user's", func() {
			BeforeEach(func() {
				renamed := binder
				renamed.Name = "Trade binder"
				fakeSvc.RenameBinderReturns(renamed, nil)
			})

			It("answers 200 with the renamed binder", func() {
				Expect(rec.Code).To(Equal(http.StatusOK))

				var got struct {
					Name string `json:"name"`
				}
				decodeBody(rec, &got)
				Expect(got.Name).To(Equal("Trade binder"))
			})

			It("passes the actor and the binder from the path", func() {
				_, passedOwnerID, passedBinderID, name := fakeSvc.RenameBinderArgsForCall(0)
				Expect(passedOwnerID).To(Equal(ownerID))
				Expect(passedBinderID).To(Equal(binderID))
				Expect(name).To(Equal("Trade binder"))
			})
		})

		When("the binder is somebody else's", func() {
			BeforeEach(func() {
				fakeSvc.RenameBinderReturns(model.Binder{}, service.ErrBinderNotFound)
			})

			It("answers 404, which tells a stranger nothing", func() {
				Expect(rec.Code).To(Equal(http.StatusNotFound))
				Expect(rec.Body.String()).To(ContainSubstring("That binder does not exist."))
			})
		})

		When("nobody is signed in", func() {
			BeforeEach(func() {
				actor = signedOut()
			})

			It("answers 401 and never reaches the service", func() {
				Expect(rec.Code).To(Equal(http.StatusUnauthorized))
				Expect(fakeSvc.RenameBinderCallCount()).To(BeZero())
			})
		})
	})

	Describe("GET /binders/{binderId}", func() {
		var target string

		BeforeEach(func() {
			target = "/binders/" + binderID.String() + "?page=1"
		})

		JustBeforeEach(func() {
			rec = do(fakeSvc, actor, http.MethodGet, target, nil)
		})

		When("the page holds cards", func() {
			var slot model.Slot

			BeforeEach(func() {
				slot = model.Slot{
					ID:            uuid.New(),
					BinderID:      binderID,
					Position:      10,
					CardID:        uuid.New(),
					SetResolution: cardmodel.SetResolutionByName,
				}
				fakeSvc.GetPageReturns(model.Page{Number: 1, Slots: []model.Slot{slot}, PageCount: 2}, nil)
			})

			It("answers 200 with a nine-pocket grid, nulls where nothing sits", func() {
				Expect(rec.Code).To(Equal(http.StatusOK))

				var got struct {
					Page      int `json:"page"`
					PageCount int `json:"pageCount"`
					Slots     []*struct {
						ID             uuid.UUID  `json:"id"`
						Position       int        `json:"position"`
						Page           int        `json:"page"`
						SlotOnPage     int        `json:"slotOnPage"`
						CardPrintingID *uuid.UUID `json:"cardPrintingId"`
						SetResolution  string     `json:"setResolution"`
					} `json:"slots"`
				}
				decodeBody(rec, &got)

				Expect(got.Page).To(Equal(1))
				Expect(got.PageCount).To(Equal(2))
				Expect(got.Slots).To(HaveLen(model.SlotsPerPage))
				Expect(got.Slots[0]).To(BeNil())
				Expect(got.Slots[1]).NotTo(BeNil())
				Expect(got.Slots[1].ID).To(Equal(slot.ID))
				Expect(got.Slots[1].Position).To(Equal(10))
				Expect(got.Slots[1].Page).To(Equal(1))
				Expect(got.Slots[1].SlotOnPage).To(Equal(1))
				Expect(got.Slots[1].CardPrintingID).To(BeNil())
				Expect(got.Slots[1].SetResolution).To(Equal("by_name"))
			})

			It("passes the actor, the binder and the page", func() {
				_, passedOwnerID, passedBinderID, page := fakeSvc.GetPageArgsForCall(0)
				Expect(passedOwnerID).To(Equal(ownerID))
				Expect(passedBinderID).To(Equal(binderID))
				Expect(page).To(Equal(1))
			})
		})

		When("the binder is empty", func() {
			BeforeEach(func() {
				fakeSvc.GetPageReturns(model.Page{Number: 0, Slots: []model.Slot{}, PageCount: 0}, nil)
			})

			It("answers 200 with an empty grid and no pages, not an error", func() {
				Expect(rec.Code).To(Equal(http.StatusOK))

				var got struct {
					PageCount int              `json:"pageCount"`
					Slots     []map[string]any `json:"slots"`
				}
				decodeBody(rec, &got)
				Expect(got.PageCount).To(BeZero())
				Expect(got.Slots).To(HaveLen(model.SlotsPerPage))
				Expect(got.Slots).To(HaveEach(BeNil()))
			})
		})

		When("no page is asked for", func() {
			BeforeEach(func() {
				target = "/binders/" + binderID.String()
			})

			It("reads the first page", func() {
				_, _, _, page := fakeSvc.GetPageArgsForCall(0)
				Expect(page).To(BeZero())
			})
		})

		When("the page number is negative", func() {
			BeforeEach(func() {
				target = "/binders/" + binderID.String() + "?page=-1"
			})

			It("is refused at the boundary, without reaching the service", func() {
				Expect(rec.Code).To(Equal(http.StatusUnprocessableEntity))
				Expect(fakeSvc.GetPageCallCount()).To(BeZero())
			})
		})

		When("the binder id in the path is not a uuid", func() {
			BeforeEach(func() {
				target = "/binders/not-a-uuid"
			})

			It("is refused at the boundary, without reaching the service", func() {
				Expect(rec.Code).To(Equal(http.StatusUnprocessableEntity))
				Expect(fakeSvc.GetPageCallCount()).To(BeZero())
			})
		})

		When("nobody is signed in", func() {
			BeforeEach(func() {
				actor = signedOut()
			})

			It("answers 401 and never reaches the service", func() {
				Expect(rec.Code).To(Equal(http.StatusUnauthorized))
				Expect(fakeSvc.GetPageCallCount()).To(BeZero())
			})
		})

		When("the service fails for a reason the table does not name", func() {
			BeforeEach(func() {
				fakeSvc.GetPageReturns(model.Page{}, errService)
			})

			It("answers an opaque 500", func() {
				Expect(rec.Code).To(Equal(http.StatusInternalServerError))
				Expect(rec.Body.String()).NotTo(ContainSubstring("service failed"))
			})
		})
	})
})

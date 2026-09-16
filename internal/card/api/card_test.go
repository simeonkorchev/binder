package api_test

import (
	"net/http"
	"net/http/httptest"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/simeonkorchev/binder/internal/card/api/apifakes"
	"github.com/simeonkorchev/binder/internal/card/model"
)

var _ = Describe("GET /cards", func() {
	var (
		blueEyes = model.Card{ID: uuid.New(), Name: "Blue-Eyes White Dragon", ImageObjectKey: nil}
	)

	var (
		fakeSvc *apifakes.FakeService
		target  string
		rec     *httptest.ResponseRecorder
	)

	BeforeEach(func() {
		fakeSvc = new(apifakes.FakeService)
		target = "/cards?q=blue&set=LOB&page=2"
	})

	JustBeforeEach(func() {
		rec = do(fakeSvc, http.MethodGet, target, nil)
	})

	When("the search finds cards", func() {
		BeforeEach(func() {
			fakeSvc.SearchCardsReturns([]model.Card{blueEyes}, nil)
		})

		It("answers 200 with the cards", func() {
			Expect(rec.Code).To(Equal(http.StatusOK))

			var got struct {
				Cards []struct {
					ID             uuid.UUID `json:"id"`
					Name           string    `json:"name"`
					ImageObjectKey *string   `json:"imageObjectKey"`
				} `json:"cards"`
			}
			decodeBody(rec, &got)

			Expect(got.Cards).To(HaveLen(1))
			Expect(got.Cards[0].ID).To(Equal(blueEyes.ID))
			Expect(got.Cards[0].Name).To(Equal("Blue-Eyes White Dragon"))
			Expect(got.Cards[0].ImageObjectKey).To(BeNil())
		})

		It("passes the query, the set and the page through, with the page size it fixes", func() {
			Expect(fakeSvc.SearchCardsCallCount()).To(Equal(1))
			_, search := fakeSvc.SearchCardsArgsForCall(0)
			Expect(search).To(Equal(model.CardSearch{
				Query:     "blue",
				SetPrefix: "LOB",
				Page:      2,
				PageSize:  50,
			}))
		})
	})

	When("no filter is given at all", func() {
		BeforeEach(func() {
			target = "/cards"
			fakeSvc.SearchCardsReturns([]model.Card{blueEyes}, nil)
		})

		It("searches everything from the first page", func() {
			Expect(rec.Code).To(Equal(http.StatusOK))
			_, search := fakeSvc.SearchCardsArgsForCall(0)
			Expect(search).To(Equal(model.CardSearch{Query: "", SetPrefix: "", Page: 1, PageSize: 50}))
		})
	})

	When("the page is past the end of the results", func() {
		BeforeEach(func() {
			fakeSvc.SearchCardsReturns(nil, nil)
		})

		It("answers 200 with an empty array, not null and not an error", func() {
			Expect(rec.Code).To(Equal(http.StatusOK))
			Expect(rec.Body.String()).To(ContainSubstring(`"cards":[]`))
		})
	})

	When("the page number is not a page number", func() {
		BeforeEach(func() { target = "/cards?page=0" })

		It("is refused at the boundary, without reaching the service", func() {
			Expect(rec.Code).To(Equal(http.StatusUnprocessableEntity))
			Expect(fakeSvc.SearchCardsCallCount()).To(BeZero())
		})
	})

	When("the service fails", func() {
		BeforeEach(func() {
			fakeSvc.SearchCardsReturns(nil, errService)
		})

		It("answers an opaque 500", func() {
			Expect(rec.Code).To(Equal(http.StatusInternalServerError))
			Expect(rec.Body.String()).NotTo(ContainSubstring("service failed"))
		})
	})
})

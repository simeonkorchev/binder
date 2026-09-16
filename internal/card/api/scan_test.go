package api_test

import (
	"net/http"
	"net/http/httptest"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/simeonkorchev/binder/internal/card/api/apifakes"
	"github.com/simeonkorchev/binder/internal/card/model"
	"github.com/simeonkorchev/binder/internal/card/service"
)

var _ = Describe("POST /scans/resolve", func() {
	var (
		imageObjectKey = "cards/dark-magician.jpg"
		darkMagician   = model.Card{
			ID:             uuid.New(),
			Name:           "Dark Magician",
			ImageObjectKey: &imageObjectKey,
		}
		lob005 = model.CardPrinting{
			ID:      uuid.New(),
			CardID:  darkMagician.ID,
			SetCode: "LOB-EN005",
			Rarity:  "Ultra Rare",
		}
	)

	var (
		fakeSvc *apifakes.FakeService
		body    map[string]any
		rec     *httptest.ResponseRecorder
	)

	BeforeEach(func() {
		fakeSvc = new(apifakes.FakeService)
		body = map[string]any{"code": "LOB-EN005", "name": "Dark Magician"}
	})

	JustBeforeEach(func() {
		rec = do(fakeSvc, http.MethodPost, "/scans/resolve", body)
	})

	When("the ladder resolves the printing exactly", func() {
		BeforeEach(func() {
			fakeSvc.ResolveScanReturns(model.MatchResult{
				Resolution: model.SetResolutionExact,
				Card:       &darkMagician,
				Printing:   &lob005,
				Candidates: nil,
			}, nil)
		})

		It("answers 200 with the card and its printing", func() {
			Expect(rec.Code).To(Equal(http.StatusOK))

			var got struct {
				Resolution string `json:"resolution"`
				Card       struct {
					ID             uuid.UUID `json:"id"`
					Name           string    `json:"name"`
					ImageObjectKey *string   `json:"imageObjectKey"`
				} `json:"card"`
				Printing struct {
					ID      uuid.UUID `json:"id"`
					CardID  uuid.UUID `json:"cardId"`
					SetCode string    `json:"setCode"`
					Rarity  string    `json:"rarity"`
				} `json:"printing"`
			}
			decodeBody(rec, &got)

			Expect(got.Resolution).To(Equal("exact"))
			Expect(got.Card.ID).To(Equal(darkMagician.ID))
			Expect(got.Card.Name).To(Equal("Dark Magician"))
			Expect(got.Card.ImageObjectKey).To(HaveValue(Equal(imageObjectKey)))
			Expect(got.Printing.ID).To(Equal(lob005.ID))
			Expect(got.Printing.CardID).To(Equal(darkMagician.ID))
			Expect(got.Printing.SetCode).To(Equal("LOB-EN005"))
			Expect(got.Printing.Rarity).To(Equal("Ultra Rare"))
		})

		It("hands the service the scan as the camera read it", func() {
			Expect(fakeSvc.ResolveScanCallCount()).To(Equal(1))
			_, scan := fakeSvc.ResolveScanArgsForCall(0)
			Expect(scan).To(Equal(model.ScanInput{Code: "LOB-EN005", Name: "Dark Magician"}))
		})

		It("serialises no candidates as an empty array, not null", func() {
			Expect(rec.Body.String()).To(ContainSubstring(`"candidates":[]`))
		})
	})

	When("the ladder could not settle the set", func() {
		BeforeEach(func() {
			fakeSvc.ResolveScanReturns(model.MatchResult{
				Resolution: model.SetResolutionUnresolved,
				Card:       &darkMagician,
				Printing:   nil,
				Candidates: []model.Candidate{{Card: darkMagician, Printing: &lob005}},
			}, nil)
		})

		It("answers 200 with a null printing and the candidates to choose from", func() {
			Expect(rec.Code).To(Equal(http.StatusOK))

			var got struct {
				Resolution string    `json:"resolution"`
				Printing   *struct{} `json:"printing"`
				Candidates []struct {
					Card struct {
						ID uuid.UUID `json:"id"`
					} `json:"card"`
					Printing *struct {
						SetCode string `json:"setCode"`
					} `json:"printing"`
				} `json:"candidates"`
			}
			decodeBody(rec, &got)

			Expect(got.Resolution).To(Equal("unresolved"))
			Expect(got.Printing).To(BeNil())
			Expect(got.Candidates).To(HaveLen(1))
			Expect(got.Candidates[0].Card.ID).To(Equal(darkMagician.ID))
			Expect(got.Candidates[0].Printing).NotTo(BeNil())
			Expect(got.Candidates[0].Printing.SetCode).To(Equal("LOB-EN005"))
		})
	})

	When("the request carries neither a code nor a name", func() {
		BeforeEach(func() {
			body = map[string]any{"code": "  ", "name": ""}
		})

		It("is refused at the boundary with 422", func() {
			Expect(rec.Code).To(Equal(http.StatusUnprocessableEntity))
		})

		It("never reaches the service", func() {
			Expect(fakeSvc.ResolveScanCallCount()).To(BeZero())
		})
	})

	When("the service finds nothing usable in a scan the boundary let through", func() {
		BeforeEach(func() {
			body = map[string]any{"code": "--", "name": ""}
			fakeSvc.ResolveScanReturns(model.MatchResult{}, service.ErrEmptyScan)
		})

		It("answers the same 422, from the table rather than the handler", func() {
			Expect(rec.Code).To(Equal(http.StatusUnprocessableEntity))
			Expect(rec.Body.String()).To(ContainSubstring("A scan needs a printed code or a card name."))
		})
	})

	When("the service fails for a reason the table does not name", func() {
		BeforeEach(func() {
			fakeSvc.ResolveScanReturns(model.MatchResult{}, errService)
		})

		It("answers an opaque 500", func() {
			Expect(rec.Code).To(Equal(http.StatusInternalServerError))
			Expect(rec.Body.String()).NotTo(ContainSubstring("service failed"))
		})
	})
})

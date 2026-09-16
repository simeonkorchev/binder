package service_test

import (
	"context"

	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/simeonkorchev/binder/internal/card/model"
	"github.com/simeonkorchev/binder/internal/card/service"
	"github.com/simeonkorchev/binder/internal/card/service/servicefakes"
)

var _ = Describe("SearchCards", func() {
	var (
		blueEyes = model.Card{ID: uuid.New(), Name: "Blue-Eyes White Dragon", ImageObjectKey: nil}
		search   = model.CardSearch{Query: "blue", SetPrefix: "LOB", Page: 2, PageSize: 50}
	)

	var (
		fakeStore *servicefakes.FakeStore
		svc       *service.Service
		cards     []model.Card
		err       error
	)

	BeforeEach(func() {
		fakeStore = new(servicefakes.FakeStore)
		svc = service.NewService(fakeStore)
	})

	JustBeforeEach(func() {
		cards, err = svc.SearchCards(context.Background(), search)
	})

	When("the store finds cards", func() {
		BeforeEach(func() {
			fakeStore.SearchCardsReturns([]model.Card{blueEyes}, nil)
		})

		It("returns them", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(cards).To(ConsistOf(blueEyes))
		})

		It("passes the search through untouched", func() {
			Expect(fakeStore.SearchCardsCallCount()).To(Equal(1))
			_, passed := fakeStore.SearchCardsArgsForCall(0)
			Expect(passed).To(Equal(search))
		})
	})

	When("the page is past the end of the results", func() {
		BeforeEach(func() {
			fakeStore.SearchCardsReturns([]model.Card{}, nil)
		})

		It("is an empty page, not an error", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(cards).To(BeEmpty())
		})
	})

	When("the store fails", func() {
		BeforeEach(func() {
			fakeStore.SearchCardsReturns(nil, errDB)
		})

		It("reports the failure", func() {
			Expect(err).To(MatchError(errDB))
			Expect(err.Error()).To(ContainSubstring("searching cards"))
			Expect(cards).To(BeNil())
		})
	})
})

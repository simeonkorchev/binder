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

// The ladder's rungs, in the order ResolveScan tries them. Every spec below
// names the rung it is pinning.
var _ = Describe("ResolveScan", func() {
	var (
		darkMagicianImage = "cards/dark-magician.jpg"
		darkMagician      = model.Card{
			ID:             uuid.New(),
			Name:           "Dark Magician",
			ImageObjectKey: &darkMagicianImage,
		}
		darkMagicianGirl = model.Card{
			ID:             uuid.New(),
			Name:           "Dark Magician Girl",
			ImageObjectKey: nil,
		}

		// Two printings of Dark Magician sharing the number EN005: the same
		// card in two sets. Matching both settles the card and not the set.
		lob005 = model.PrintedCard{
			Card: darkMagician,
			Printing: model.CardPrinting{
				ID: uuid.New(), CardID: darkMagician.ID, SetCode: "LOB-EN005", Rarity: "Ultra Rare",
			},
		}
		sdy005 = model.PrintedCard{
			Card: darkMagician,
			Printing: model.CardPrinting{
				ID: uuid.New(), CardID: darkMagician.ID, SetCode: "SDY-EN005", Rarity: "Common",
			},
		}
		// A printing of a *different* card under the same number. This is the
		// ambiguity case: the number alone settles nothing at all.
		mfc005 = model.PrintedCard{
			Card: darkMagicianGirl,
			Printing: model.CardPrinting{
				ID: uuid.New(), CardID: darkMagicianGirl.ID, SetCode: "MFC-EN005", Rarity: "Secret Rare",
			},
		}
	)

	var (
		fakeStore *servicefakes.FakeStore
		svc       *service.Service
		scan      model.ScanInput
		result    model.MatchResult
		err       error
	)

	BeforeEach(func() {
		fakeStore = new(servicefakes.FakeStore)
		svc = service.NewService(fakeStore)
		scan = model.ScanInput{Code: "LOB-EN005", Name: "Dark Magician"}
	})

	JustBeforeEach(func() {
		result, err = svc.ResolveScan(context.Background(), scan)
	})

	// The equivalence db/migrations/003_binders.sql states as a CHECK. Asserted
	// after every spec, so no rung can produce a result a binder slot would
	// have to refuse.
	//
	// The second assertion is the one that keeps Outcome honest. Every rung
	// states its own outcome where it concludes, so nothing in the ladder
	// derives it from the other fields — which is exactly why a test has to,
	// independently, and compare. Together the specs below reach all five
	// shapes the ladder can produce, so this runs over every one of them.
	AfterEach(func() {
		if err != nil {
			return
		}
		Expect(model.RequiresPrinting(result.Resolution)).To(
			Equal(result.Printing != nil),
			"a resolution requiring a printing must carry one, and no other may",
		)
		Expect(result.Outcome).To(
			Equal(outcomeOfShape(result)),
			"the stated outcome must agree with the shape of the result it came with",
		)
	})

	When("the scan carries neither a usable code nor a name", func() {
		BeforeEach(func() {
			scan = model.ScanInput{Code: "", Name: ""}
		})

		It("refuses the scan", func() {
			Expect(err).To(MatchError(service.ErrEmptyScan))
		})

		It("does not reach the store", func() {
			Expect(fakeStore.FindPrintingsBySetCodeCallCount()).To(BeZero())
			Expect(fakeStore.FindPrintingsBySetNumberCallCount()).To(BeZero())
			Expect(fakeStore.FindCardsByNameCallCount()).To(BeZero())
		})

		Context("because the code carried no letter or digit at all", func() {
			BeforeEach(func() {
				scan = model.ScanInput{Code: " -- ", Name: "   "}
			})

			It("refuses the scan too", func() {
				Expect(err).To(MatchError(service.ErrEmptyScan))
			})
		})
	})

	// Rung 1.
	When("the whole printed code matches exactly one printing", func() {
		BeforeEach(func() {
			fakeStore.FindPrintingsBySetCodeReturns([]model.PrintedCard{lob005}, nil)
		})

		It("resolves the printing exactly", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Resolution).To(Equal(model.SetResolutionExact))
			Expect(result.Card).To(HaveValue(Equal(darkMagician)))
			Expect(result.Printing).To(HaveValue(Equal(lob005.Printing)))
			Expect(result.Candidates).To(BeEmpty())
		})

		It("asks for the code reassembled from the scan", func() {
			Expect(fakeStore.FindPrintingsBySetCodeCallCount()).To(Equal(1))
			_, setCode := fakeStore.FindPrintingsBySetCodeArgsForCall(0)
			Expect(setCode).To(Equal("LOB-EN005"))
		})

		It("stops the ladder there", func() {
			Expect(fakeStore.FindPrintingsBySetPrefixAndNumberCallCount()).To(BeZero())
			Expect(fakeStore.FindPrintingsBySetNumberCallCount()).To(BeZero())
			Expect(fakeStore.FindCardsByNameCallCount()).To(BeZero())
		})

		Context("and OCR mangled the separator and the case", func() {
			BeforeEach(func() {
				scan = model.ScanInput{Code: " lob en005 ", Name: "Dark Magician"}
			})

			It("normalises the code before matching it", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Resolution).To(Equal(model.SetResolutionExact))
				_, setCode := fakeStore.FindPrintingsBySetCodeArgsForCall(0)
				Expect(setCode).To(Equal("LOB-EN005"))
			})
		})
	})

	// Rung 2 — the rung that saves a misread prefix tail.
	When("the exact code matches nothing but the number and a prefix of the set's prefix do", func() {
		BeforeEach(func() {
			scan = model.ScanInput{Code: "LO-EN005", Name: "Dark Magician"}
			fakeStore.FindPrintingsBySetCodeReturns(nil, nil)
			fakeStore.FindPrintingsBySetPrefixAndNumberReturns([]model.PrintedCard{lob005}, nil)
		})

		It("resolves by prefix and number", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Resolution).To(Equal(model.SetResolutionByPrefixAndNumber))
			Expect(result.Card).To(HaveValue(Equal(darkMagician)))
			Expect(result.Printing).To(HaveValue(Equal(lob005.Printing)))
			Expect(result.Candidates).To(BeEmpty())
		})

		It("asks for the two halves the scan read", func() {
			Expect(fakeStore.FindPrintingsBySetPrefixAndNumberCallCount()).To(Equal(1))
			_, prefix, number := fakeStore.FindPrintingsBySetPrefixAndNumberArgsForCall(0)
			Expect(prefix).To(Equal("LO"))
			Expect(number).To(Equal("EN005"))
		})

		It("stops before the number rung", func() {
			Expect(fakeStore.FindPrintingsBySetNumberCallCount()).To(BeZero())
			Expect(fakeStore.FindCardsByNameCallCount()).To(BeZero())
		})
	})

	// Rung 3.
	When("only the number alone matches, and it matches one printing", func() {
		BeforeEach(func() {
			fakeStore.FindPrintingsBySetCodeReturns(nil, nil)
			fakeStore.FindPrintingsBySetPrefixAndNumberReturns(nil, nil)
			fakeStore.FindPrintingsBySetNumberReturns([]model.PrintedCard{sdy005}, nil)
		})

		It("resolves by number", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Resolution).To(Equal(model.SetResolutionByNumber))
			Expect(result.Card).To(HaveValue(Equal(darkMagician)))
			Expect(result.Printing).To(HaveValue(Equal(sdy005.Printing)))
		})

		It("asks for the number the scan read", func() {
			Expect(fakeStore.FindPrintingsBySetNumberCallCount()).To(Equal(1))
			_, number := fakeStore.FindPrintingsBySetNumberArgsForCall(0)
			Expect(number).To(Equal("EN005"))
		})

		It("does not fall through to the name", func() {
			Expect(fakeStore.FindCardsByNameCallCount()).To(BeZero())
		})

		Context("and the prefix did not survive OCR at all", func() {
			BeforeEach(func() {
				scan = model.ScanInput{Code: "EN005", Name: "Dark Magician"}
			})

			It("skips the two rungs that need a prefix", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Resolution).To(Equal(model.SetResolutionByNumber))
				Expect(fakeStore.FindPrintingsBySetCodeCallCount()).To(BeZero())
				Expect(fakeStore.FindPrintingsBySetPrefixAndNumberCallCount()).To(BeZero())
			})
		})
	})

	// The ambiguity cases: a rung that matched several printings settles the
	// set for none of them.
	When("a rung matches several printings of one card", func() {
		BeforeEach(func() {
			fakeStore.FindPrintingsBySetCodeReturns(nil, nil)
			fakeStore.FindPrintingsBySetPrefixAndNumberReturns(nil, nil)
			fakeStore.FindPrintingsBySetNumberReturns([]model.PrintedCard{lob005, sdy005}, nil)
		})

		It("settles the card and leaves the set open", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Outcome).To(Equal(model.ScanOutcomeAmbiguous))
			Expect(result.Resolution).To(Equal(model.SetResolutionUnresolved))
			Expect(result.Card).To(HaveValue(Equal(darkMagician)))
			Expect(result.Printing).To(BeNil())
		})

		It("offers both printings for the user to choose between", func() {
			Expect(result.Candidates).To(ConsistOf(
				model.Candidate{Card: darkMagician, Printing: &lob005.Printing},
				model.Candidate{Card: darkMagician, Printing: &sdy005.Printing},
			))
		})

		It("does not fall through to the name rung", func() {
			Expect(fakeStore.FindCardsByNameCallCount()).To(BeZero())
		})
	})

	When("the number alone matches printings of different cards", func() {
		BeforeEach(func() {
			scan = model.ScanInput{Code: "EN005", Name: "Dark Magician"}
			fakeStore.FindPrintingsBySetNumberReturns([]model.PrintedCard{lob005, mfc005}, nil)
		})

		It("settles neither the card nor the set", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Outcome).To(Equal(model.ScanOutcomeAmbiguous))
			Expect(result.Resolution).To(Equal(model.SetResolutionUnresolved))
			Expect(result.Card).To(BeNil())
			Expect(result.Printing).To(BeNil())
		})

		It("offers both cards' printings", func() {
			Expect(result.Candidates).To(ConsistOf(
				model.Candidate{Card: darkMagician, Printing: &lob005.Printing},
				model.Candidate{Card: darkMagicianGirl, Printing: &mfc005.Printing},
			))
		})
	})

	// Rung 4.
	When("no code rung matches and one card's name is clearly the best", func() {
		BeforeEach(func() {
			fakeStore.FindPrintingsBySetNumberReturns(nil, nil)
			fakeStore.FindCardsByNameReturns([]model.NameMatch{
				{Card: darkMagician, Similarity: 0.9},
				{Card: darkMagicianGirl, Similarity: 0.6},
			}, nil)
		})

		It("resolves the card by name and no set", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Outcome).To(Equal(model.ScanOutcomeCardOnly))
			Expect(result.Resolution).To(Equal(model.SetResolutionByName))
			Expect(result.Card).To(HaveValue(Equal(darkMagician)))
			Expect(result.Printing).To(BeNil())
			Expect(result.Candidates).To(BeEmpty())
		})

		It("asks for the trimmed name with the ladder's own threshold", func() {
			Expect(fakeStore.FindCardsByNameCallCount()).To(Equal(1))
			_, name, threshold, limit := fakeStore.FindCardsByNameArgsForCall(0)
			Expect(name).To(Equal("Dark Magician"))
			Expect(threshold).To(BeNumerically("==", 0.3))
			Expect(limit).To(Equal(5))
		})
	})

	When("the two best names score identically", func() {
		BeforeEach(func() {
			fakeStore.FindCardsByNameReturns([]model.NameMatch{
				{Card: darkMagician, Similarity: 0.8},
				{Card: darkMagicianGirl, Similarity: 0.8},
			}, nil)
		})

		It("refuses to pick a winner", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Outcome).To(Equal(model.ScanOutcomeAmbiguous))
			Expect(result.Resolution).To(Equal(model.SetResolutionUnresolved))
			Expect(result.Card).To(BeNil())
			Expect(result.Printing).To(BeNil())
		})

		It("offers the tied cards, with no printing on either", func() {
			Expect(result.Candidates).To(ConsistOf(
				model.Candidate{Card: darkMagician, Printing: nil},
				model.Candidate{Card: darkMagicianGirl, Printing: nil},
			))
		})
	})

	// Rung 5.
	When("no rung matches anything", func() {
		BeforeEach(func() {
			fakeStore.FindCardsByNameReturns(nil, nil)
		})

		It("is unresolved, and that is not an error", func() {
			Expect(err).NotTo(HaveOccurred())
			// The point of the outcome field: this answers the same
			// `unresolved` resolution as the two ambiguous specs above, and it
			// is a different outcome, because the user's next move is a search
			// rather than a choice.
			Expect(result.Outcome).To(Equal(model.ScanOutcomeNoMatch))
			Expect(result.Resolution).To(Equal(model.SetResolutionUnresolved))
			Expect(result.Card).To(BeNil())
			Expect(result.Printing).To(BeNil())
			Expect(result.Candidates).To(BeEmpty())
		})

		It("walked every rung on the way down", func() {
			Expect(fakeStore.FindPrintingsBySetCodeCallCount()).To(Equal(1))
			Expect(fakeStore.FindPrintingsBySetPrefixAndNumberCallCount()).To(Equal(1))
			Expect(fakeStore.FindPrintingsBySetNumberCallCount()).To(Equal(1))
			Expect(fakeStore.FindCardsByNameCallCount()).To(Equal(1))
		})

		Context("and the scan carried no name to fall back on", func() {
			BeforeEach(func() {
				scan = model.ScanInput{Code: "LOB-EN005", Name: ""}
			})

			It("is unresolved without asking the name rung", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Resolution).To(Equal(model.SetResolutionUnresolved))
				Expect(fakeStore.FindCardsByNameCallCount()).To(BeZero())
			})
		})
	})

	When("the exact rung's lookup fails", func() {
		BeforeEach(func() {
			fakeStore.FindPrintingsBySetCodeReturns(nil, errDB)
		})

		It("reports the failure rather than falling to a lower rung", func() {
			Expect(err).To(MatchError(errDB))
			Expect(err.Error()).To(ContainSubstring("matching the scanned code exactly"))
			Expect(fakeStore.FindPrintingsBySetPrefixAndNumberCallCount()).To(BeZero())
		})
	})

	When("the prefix-and-number rung's lookup fails", func() {
		BeforeEach(func() {
			fakeStore.FindPrintingsBySetPrefixAndNumberReturns(nil, errDB)
		})

		It("reports the failure", func() {
			Expect(err).To(MatchError(errDB))
			Expect(err.Error()).To(ContainSubstring("matching the scanned set prefix and number"))
			Expect(fakeStore.FindPrintingsBySetNumberCallCount()).To(BeZero())
		})
	})

	When("the number rung's lookup fails", func() {
		BeforeEach(func() {
			fakeStore.FindPrintingsBySetNumberReturns(nil, errDB)
		})

		It("reports the failure", func() {
			Expect(err).To(MatchError(errDB))
			Expect(err.Error()).To(ContainSubstring("matching the scanned set number"))
			Expect(fakeStore.FindCardsByNameCallCount()).To(BeZero())
		})
	})

	When("the name rung's lookup fails", func() {
		BeforeEach(func() {
			fakeStore.FindCardsByNameReturns(nil, errDB)
		})

		It("reports the failure", func() {
			Expect(err).To(MatchError(errDB))
			Expect(err.Error()).To(ContainSubstring("matching the scanned name"))
		})
	})
})

// outcomeOfShape works out what a result's Outcome has to be from the rest of
// its fields, so the AfterEach above can hold the ladder to it. It is the
// derivation the mobile app used to do for itself (lib/flaggedScans.ts), kept
// here as a test oracle and nowhere in production: the server states the
// outcome, and this is what proves the statement true.
func outcomeOfShape(result model.MatchResult) model.ScanOutcome {
	switch {
	case len(result.Candidates) > 0:
		return model.ScanOutcomeAmbiguous
	case result.Card == nil:
		return model.ScanOutcomeNoMatch
	case result.Printing == nil:
		return model.ScanOutcomeCardOnly
	default:
		return model.ScanOutcomeResolved
	}
}

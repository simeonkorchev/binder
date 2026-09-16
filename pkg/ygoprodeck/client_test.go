package ygoprodeck_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/simeonkorchev/binder/pkg/ygoprodeck"
)

// The API is unreachable from this repo's containers (403 at the egress
// proxy), so every spec here drives the client over a stub Doer against the
// committed fixture. What they pin is the *assumed* contract in the package
// doc; specs/001-binder-mvp/VERIFY-YGOPRODECK.md is how a human confirms it.

const testURL = "https://cards.test/api/v7/cardinfo.php"

var errTransport = errors.New("transport refused")

// stubDoer is the fake half of the Doer seam: it records what was requested
// and answers with whatever the spec set up.
type stubDoer struct {
	requests []*http.Request
	respond  func(*http.Request) (*http.Response, error)
}

func (s *stubDoer) Do(req *http.Request) (*http.Response, error) {
	s.requests = append(s.requests, req)

	return s.respond(req)
}

// countingBody reports whether the client closed it.
type countingBody struct {
	reader io.Reader
	mu     sync.Mutex
	closed bool
}

func (b *countingBody) Read(p []byte) (int, error) { return b.reader.Read(p) }

func (b *countingBody) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.closed = true

	return nil
}

func (b *countingBody) isClosed() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.closed
}

func fixtureDump() string {
	raw, err := os.ReadFile("testdata/cardinfo_sample.json")
	Expect(err).NotTo(HaveOccurred())

	return string(raw)
}

func cardNamed(cards []ygoprodeck.Card, name string) ygoprodeck.Card {
	for _, card := range cards {
		if card.Name == name {
			return card
		}
	}
	Fail("no card named " + name + " in the decoded dump")

	return ygoprodeck.Card{}
}

var _ = Describe("Client", func() {
	var (
		ctx    context.Context
		doer   *stubDoer
		body   *countingBody
		client *ygoprodeck.Client
		cards  []ygoprodeck.Card
		err    error
	)

	respondWith := func(status int, payload string) {
		body = &countingBody{reader: strings.NewReader(payload)}
		doer.respond = func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: status, Body: body}, nil
		}
	}

	BeforeEach(func() {
		ctx = context.Background()
		doer = &stubDoer{}
		respondWith(http.StatusOK, fixtureDump())
		client = ygoprodeck.NewClient(ygoprodeck.Config{CardInfoURL: testURL, HTTPClient: doer})
	})

	JustBeforeEach(func() {
		cards, err = client.FetchAllCards(ctx)
	})

	When("the API answers with the documented envelope", func() {
		It("returns every card in the dump", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(cards).To(HaveLen(5))
			names := make([]string, 0, len(cards))
			for _, card := range cards {
				names = append(names, card.Name)
			}
			Expect(names).To(ConsistOf(
				"Blue-Eyes White Dragon", "Dark Magician", "Mirror Force",
				"Obelisk the Tormentor", "Kuriboh",
			))
		})

		It("asks for the whole dump: one GET, no query parameters", func() {
			Expect(doer.requests).To(HaveLen(1))
			Expect(doer.requests[0].Method).To(Equal(http.MethodGet))
			Expect(doer.requests[0].URL.String()).To(Equal(testURL))
			Expect(doer.requests[0].URL.RawQuery).To(BeEmpty())
		})

		It("identifies itself and asks for JSON", func() {
			Expect(doer.requests[0].Header.Get("User-Agent")).To(ContainSubstring("binder-cardimport"))
			Expect(doer.requests[0].Header.Get("Accept")).To(Equal("application/json"))
		})

		It("closes the response body", func() {
			Expect(body.isClosed()).To(BeTrue())
		})

		// Completeness: every field of every wire type, from one card that has
		// all of them set. A field added to types.go without being decoded
		// fails here (000-principles.md section 9).
		It("populates every field of Card, CardSet and CardImage", func() {
			card := cardNamed(cards, "Blue-Eyes White Dragon")
			Expect(card.ID).To(Equal(int64(89631139)))
			Expect(card.Name).To(Equal("Blue-Eyes White Dragon"))

			Expect(card.Sets).To(HaveLen(4))
			Expect(card.Sets[0].Name).To(Equal("Legend of Blue Eyes White Dragon"))
			Expect(card.Sets[0].Code).To(Equal("LOB-EN001"))
			Expect(card.Sets[0].Rarity).To(Equal("Ultra Rare"))

			Expect(card.Images).To(HaveLen(2))
			Expect(card.Images[0].ID).To(Equal(int64(89631139)))
			Expect(card.Images[0].URL).To(Equal("https://images.ygoprodeck.com/images/cards/89631139.jpg"))
			Expect(card.Images[0].SmallURL).
				To(Equal("https://images.ygoprodeck.com/images/cards_small/89631139.jpg"))
		})

		DescribeTable("decodes a card's printings verbatim, including the ones the importer will reject",
			func(name string, want []ygoprodeck.CardSet) {
				Expect(cardNamed(cards, name).Sets).To(Equal(want))
			},
			Entry("one code printed at two rarities, plus a duplicate and a multi-hyphen code",
				"Blue-Eyes White Dragon", []ygoprodeck.CardSet{
					{Name: "Legend of Blue Eyes White Dragon", Code: "LOB-EN001", Rarity: "Ultra Rare"},
					{Name: "Legend of Blue Eyes White Dragon", Code: "LOB-EN001", Rarity: "Secret Rare"},
					{Name: "Legend of Blue Eyes White Dragon", Code: "LOB-EN001", Rarity: "Ultra Rare"},
					{Name: "Starter Deck: Kaiba", Code: "SDK-EN-A01", Rarity: "Common"},
				}),
			Entry("a lower-case code and a code with no hyphen at all",
				"Dark Magician", []ygoprodeck.CardSet{
					{Name: "Legend of Blue Eyes White Dragon", Code: "LOB-EN005", Rarity: "Ultra Rare"},
					{Name: "Starter Deck: Yugi", Code: "sdy-en006", Rarity: "Common"},
					{Name: "Tournament Pack", Code: "PROMO", Rarity: "Ultra Rare"},
				}),
			Entry("codes missing a prefix and missing a number",
				"Mirror Force", []ygoprodeck.CardSet{
					{Name: "Metal Raiders", Code: "MRD-EN138", Rarity: "Ultra Rare"},
					{Name: "Metal Raiders", Code: "-EN139", Rarity: "Rare"},
					{Name: "Metal Raiders", Code: "MRD-", Rarity: "Rare"},
				}),
			Entry("an empty rarity", "Kuriboh", []ygoprodeck.CardSet{
				{Name: "Legend of Blue Eyes White Dragon: Reprint", Code: "LOB-EN071", Rarity: "Rare"},
				{Name: "Shonen Jump Promo", Code: "JMP-EN001", Rarity: ""},
			}),
			Entry("a card with no printings at all", "Obelisk the Tormentor", []ygoprodeck.CardSet{}),
		)
	})

	When("the response carries no data array", func() {
		BeforeEach(func() {
			respondWith(http.StatusOK, `{"error":"no card matching your query"}`)
		})

		// 000-principles.md section 8b: empty is not an error.
		It("returns no cards and no error", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(cards).To(BeEmpty())
		})
	})

	When("the API answers with a status other than 200", func() {
		BeforeEach(func() {
			respondWith(http.StatusTooManyRequests, "")
		})

		It("reports the status and returns no cards", func() {
			Expect(err).To(MatchError(ygoprodeck.ErrUnexpectedStatus))
			Expect(err.Error()).To(ContainSubstring("429"))
			Expect(cards).To(BeNil())
		})

		It("still closes the response body", func() {
			Expect(body.isClosed()).To(BeTrue())
		})
	})

	When("the body is not the documented JSON", func() {
		BeforeEach(func() {
			respondWith(http.StatusOK, "<html><body>maintenance</body></html>")
		})

		It("fails with a decode error rather than panicking", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("decoding the card dump"))
			Expect(cards).To(BeNil())
		})
	})

	When("the transport fails", func() {
		BeforeEach(func() {
			doer.respond = func(*http.Request) (*http.Response, error) { return nil, errTransport }
		})

		It("wraps the transport error and does not retry", func() {
			Expect(err).To(MatchError(errTransport))
			Expect(err.Error()).To(ContainSubstring("requesting the card dump"))
			Expect(doer.requests).To(HaveLen(1))
		})
	})

	When("the context is already cancelled", func() {
		BeforeEach(func() {
			cancelled, cancel := context.WithCancel(context.Background())
			cancel()
			ctx = cancelled
			doer.respond = func(req *http.Request) (*http.Response, error) {
				return nil, req.Context().Err()
			}
		})

		It("gives up without decoding anything", func() {
			Expect(err).To(MatchError(context.Canceled))
			Expect(cards).To(BeNil())
		})
	})
})

var _ = Describe("NewClient", func() {
	When("the config is empty", func() {
		It("falls back to the documented endpoint", func() {
			doer := &stubDoer{respond: func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"data":[]}`))}, nil
			}}
			client := ygoprodeck.NewClient(ygoprodeck.Config{HTTPClient: doer})

			_, err := client.FetchAllCards(context.Background())

			Expect(err).NotTo(HaveOccurred())
			Expect(doer.requests[0].URL.String()).To(Equal(ygoprodeck.DefaultCardInfoURL))
		})
	})
})

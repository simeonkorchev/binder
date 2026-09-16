package importer_test

import (
	"context"
	"io"
	"net/http"
	"os"
	"strings"

	. "github.com/onsi/gomega"
	"github.com/simeonkorchev/binder/cmd/cardimport/model"
	"github.com/simeonkorchev/binder/pkg/ygoprodeck"
)

// The dump these specs import is pkg/ygoprodeck/testdata/cardinfo_sample.json,
// read through the real client over a stub transport. Its README makes it the
// single fixture behind both the decode specs there and the import specs here,
// so the two cannot drift apart — and going through the real decode means a
// change to the wire types shows up in both.
//
// The live API is never contacted: db.ygoprodeck.com is answered with 403 by
// the egress proxy in this repository's containers, and nothing in this suite
// opens a socket.
const fixturePath = "../../../pkg/ygoprodeck/testdata/cardinfo_sample.json"

// stubDoer answers every request with one canned body, which is all the
// importer needs: the dump arrives in a single response (assumption A1).
type stubDoer struct {
	body string
}

func (s stubDoer) Do(*http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(s.body)),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}, nil
}

func fixtureCards() []ygoprodeck.Card {
	raw, err := os.ReadFile(fixturePath)
	Expect(err).NotTo(HaveOccurred())

	client := ygoprodeck.NewClient(ygoprodeck.Config{HTTPClient: stubDoer{body: string(raw)}})
	cards, err := client.FetchAllCards(context.Background())
	Expect(err).NotTo(HaveOccurred())

	return cards
}

// allBatches returns every batch the store was handed, in order.
func allBatches(recorded func(int) (context.Context, model.Batch), calls int) []model.Batch {
	batches := make([]model.Batch, 0, calls)
	for call := range calls {
		_, batch := recorded(call)
		batches = append(batches, batch)
	}

	return batches
}

// printingsOf flattens the printings of every batch, which is what a spec that
// does not care how the dump was sliced wants to assert over.
func printingsOf(batches []model.Batch) []model.PrintingRow {
	var printings []model.PrintingRow
	for _, batch := range batches {
		printings = append(printings, batch.Printings...)
	}

	return printings
}

func cardsOf(batches []model.Batch) []model.CardRow {
	var cards []model.CardRow
	for _, batch := range batches {
		cards = append(cards, batch.Cards...)
	}

	return cards
}

func setsOf(batches []model.Batch) []model.SetRow {
	var sets []model.SetRow
	for _, batch := range batches {
		sets = append(sets, batch.Sets...)
	}

	return sets
}

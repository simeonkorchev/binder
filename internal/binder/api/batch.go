package api

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
	"github.com/simeonkorchev/binder/internal/binder/model"
)

type addSlotsInput struct {
	BinderID uuid.UUID `path:"binderId"`
	Body     addSlotsBody
}

// addSlotsBody is a whole reviewed scan session: the cards to append, in the
// order they should sit in the binder.
//
// maxItems repeats model.MaxSlotsPerBatch as a literal because a struct tag
// cannot read a constant. batch_internal_test.go pins the two together, so the
// day the bound moves and the tag does not is a failing build rather than a
// contract that lies about what it accepts.
type addSlotsBody struct {
	Cards []slotCardBody `json:"cards" maxItems:"450" doc:"The cards to append, in order. At most 450 — a longer commit is split by the client."` //nolint:lll // one doc string; wrapping a struct tag changes it.
}

// Resolve is the boundary guard for the batch. It checks every entry rather
// than stopping at the first, so a client fixing a sixty-card commit is told
// about all of it at once, and each error points at the entry it came from.
//
// The length is huma's to enforce from maxItems above; the service checks it
// again, because this guard is not a licence for the layer below to assume
// (000-principles.md section 10).
func (b addSlotsBody) Resolve(huma.Context) []error {
	problems := make([]error, 0, len(b.Cards))
	for i, card := range b.Cards {
		problems = append(problems, card.validate("body.cards["+strconv.Itoa(i)+"]")...)
	}
	return problems
}

// addSlotsOutput carries the stored slots under a name rather than as a bare
// array, so a field can be added beside them later without changing the shape
// a client already parses.
type addSlotsOutput struct {
	Body addSlotsResult
}

type addSlotsResult struct {
	// Slots is what was written, in position order — the same order the cards
	// arrived in. Empty, never null, for a commit of no cards.
	Slots []slotBody `json:"slots"`
}

func registerBatchEndpoints(api huma.API, svc SlotService, actor ActorFunc) {
	huma.Register(api,
		newOp(http.MethodPost, "/binders/{binderId}/slots/batch", "add-binder-slots",
			"Append a reviewed scan session to a binder: every card, or none.", http.StatusCreated),
		func(ctx context.Context, req *addSlotsInput) (*addSlotsOutput, error) {
			ownerID, err := actor(ctx)
			if err != nil {
				return nil, errNoActor
			}

			slots, err := svc.AddSlots(ctx, ownerID, req.BinderID, model.AddSlotsInput{
				Cards: toSlotCards(req.Body.Cards),
			})
			if err != nil {
				return nil, handleErr(ctx, fmt.Errorf("adding cards to the binder: %w", err))
			}
			return &addSlotsOutput{Body: addSlotsResult{Slots: toSlotBodies(slots)}}, nil
		})
}

// toSlotCards maps the request's cards to the service's, in order. Order is the
// payload here — it is what decides the position each card lands at — so this
// never sorts, groups or de-duplicates.
func toSlotCards(cards []slotCardBody) []model.SlotCard {
	mapped := make([]model.SlotCard, 0, len(cards))
	for _, card := range cards {
		mapped = append(mapped, card.toSlotCard())
	}
	return mapped
}

// toSlotBodies always returns a slice, so a commit of no cards serialises its
// slots as [] rather than null (000-principles.md section 8b).
func toSlotBodies(slots []model.Slot) []slotBody {
	mapped := make([]slotBody, 0, len(slots))
	for _, slot := range slots {
		mapped = append(mapped, toSlotBody(slot))
	}
	return mapped
}

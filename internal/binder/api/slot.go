package api

import (
	"context"
	"fmt"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
	"github.com/simeonkorchev/binder/internal/binder/model"
	cardmodel "github.com/simeonkorchev/binder/internal/card/model"
)

// errNoActor is what a handler returns when ActorFunc cannot name the user.
// Built once: there is nothing request-specific in a 401.
var errNoActor = huma.Error401Unauthorized("Sign in to use your binders.")

// SlotService is the half of the domain that owns the cards in a binder.
//
// It is faked through api.Service, which embeds it.
type SlotService interface {
	AddSlot(ctx context.Context, ownerID, binderID uuid.UUID, input model.AddSlotInput) (model.Slot, error)
	AddSlots(ctx context.Context, ownerID, binderID uuid.UUID, input model.AddSlotsInput) ([]model.Slot, error)
	MoveSlot(ctx context.Context, ownerID, binderID uuid.UUID, input model.MoveSlotInput) error
	RemoveSlot(ctx context.Context, ownerID, binderID, slotID uuid.UUID) error
}

// setResolution is cardmodel.SetResolution on the wire. It is its own type so
// the OpenAPI enum is generated from cardmodel.SetResolutions() rather than
// re-typed into a struct tag that the next rung added would not update.
type setResolution cardmodel.SetResolution

// Schema declares the enum from the one list of rungs.
func (setResolution) Schema(huma.Registry) *huma.Schema {
	resolutions := cardmodel.SetResolutions()
	values := make([]any, 0, len(resolutions))
	for _, resolution := range resolutions {
		values = append(values, string(resolution))
	}

	return &huma.Schema{
		Type:        huma.TypeString,
		Enum:        values,
		Description: "How this card's set was determined when it was scanned.",
	}
}

// slotBody is one card in one pocket of a binder.
type slotBody struct {
	ID       uuid.UUID `json:"id"`
	Position int       `json:"position"`
	// Page and SlotOnPage are derived from Position. They are sent rather than
	// left to the client to recompute, so the 3x3 grid is one derivation in one
	// place (model.SlotsPerPage) instead of one per client.
	Page       int       `json:"page"`
	SlotOnPage int       `json:"slotOnPage"`
	CardID     uuid.UUID `json:"cardId"`
	// CardPrintingID is null exactly when SetResolution did not determine a
	// set, which is the same equivalence the binder_slots CHECK enforces.
	CardPrintingID *uuid.UUID    `json:"cardPrintingId"`
	SetResolution  setResolution `json:"setResolution"`
}

func toSlotBody(slot model.Slot) slotBody {
	return slotBody{
		ID:             slot.ID,
		Position:       slot.Position,
		Page:           slot.Page(),
		SlotOnPage:     slot.SlotOnPage(),
		CardID:         slot.CardID,
		CardPrintingID: slot.CardPrintingID,
		SetResolution:  setResolution(slot.SetResolution),
		// BinderID, CreatedAt and UpdatedAt are deliberately not carried
		// across. The binder id is in the path of every request that can
		// produce a slot, so repeating it in the body would be one more thing
		// that can disagree with it; the timestamps are the row's own
		// bookkeeping and nothing in the app shows when a card was filed.
		// mapper_internal_test.go states the same three omissions.
	}
}

type addSlotInput struct {
	BinderID uuid.UUID `path:"binderId"`
	Body     addSlotBody
}

// slotCardBody is a card to put into a binder, as the review sheet confirmed
// it: everything a slot records except where it goes. Both writes carry it, so
// the rule that the resolution and the printing must agree is checked by one
// function for both.
type slotCardBody struct {
	CardID         uuid.UUID     `json:"cardId"`
	CardPrintingID *uuid.UUID    `json:"cardPrintingId" required:"false"`
	SetResolution  setResolution `json:"setResolution"`
}

func (b slotCardBody) toSlotCard() model.SlotCard {
	return model.SlotCard{
		CardID:         b.CardID,
		CardPrintingID: b.CardPrintingID,
		SetResolution:  cardmodel.SetResolution(b.SetResolution),
	}
}

// validate rejects a card whose resolution and printing disagree, so the client
// is told which field is wrong rather than getting a bare 422 from the service.
// location is where in the request body this card sits, so a batch can point at
// the entry that is wrong rather than at the batch.
//
// The service checks the same thing again: this guard is not a licence for the
// layer below to assume (000-principles.md section 10).
func (b slotCardBody) validate(location string) []error {
	resolution := cardmodel.SetResolution(b.SetResolution)
	if !cardmodel.ValidSetResolution(resolution) {
		// The enum in the schema already refused anything else; this is the
		// zero value, which is not a resolution.
		return []error{&huma.ErrorDetail{
			Message:  "setResolution has to be a way a card's set can be resolved.",
			Location: location + ".setResolution",
			Value:    string(b.SetResolution),
		}}
	}

	requiresPrinting := cardmodel.RequiresPrinting(resolution)
	if requiresPrinting && b.CardPrintingID == nil {
		return []error{&huma.ErrorDetail{
			Message:  "A card whose set was determined has to name the printing it was determined to be.",
			Location: location + ".cardPrintingId",
			Value:    nil,
		}}
	}
	if !requiresPrinting && b.CardPrintingID != nil {
		return []error{&huma.ErrorDetail{
			Message:  "A card whose set was not determined cannot name a printing.",
			Location: location + ".cardPrintingId",
			Value:    b.CardPrintingID,
		}}
	}
	return nil
}

// addSlotBody is one card and where it goes.
//
// It repeats slotCardBody's three fields rather than embedding it: huma does
// not flatten an anonymous embedded struct into the parent's schema — the
// embedded fields simply disappear from it, and `additionalProperties: false`
// then rejects every request that sends them. The rule about those three fields
// is what must not be duplicated, and it is not: both bodies validate through
// slotCardBody.validate and convert through slotCardBody.toSlotCard.
type addSlotBody struct {
	CardID         uuid.UUID     `json:"cardId"`
	CardPrintingID *uuid.UUID    `json:"cardPrintingId" required:"false"`
	SetResolution  setResolution `json:"setResolution"`
	// Position is where the card goes. Absent appends it after the last card,
	// which is what a scan does.
	Position *int `json:"position" required:"false" minimum:"0"`
}

// card is the part of this body a slot records, in the shape both writes share.
func (b addSlotBody) card() slotCardBody {
	return slotCardBody{
		CardID:         b.CardID,
		CardPrintingID: b.CardPrintingID,
		SetResolution:  b.SetResolution,
	}
}

// Resolve is the boundary guard for the single-card write.
func (b addSlotBody) Resolve(huma.Context) []error {
	return b.card().validate("body")
}

type slotOutput struct {
	Body slotBody
}

type moveSlotInput struct {
	BinderID uuid.UUID `path:"binderId"`
	Body     struct {
		SlotID     uuid.UUID `json:"slotId"`
		ToPosition int       `json:"toPosition" minimum:"0" doc:"Where the card should end up."`
	}
}

type removeSlotInput struct {
	BinderID uuid.UUID `path:"binderId"`
	SlotID   uuid.UUID `path:"slotId"`
}

func registerSlotEndpoints(api huma.API, svc SlotService, actor ActorFunc) {
	huma.Register(api,
		newOp(http.MethodPost, "/binders/{binderId}/slots", "add-binder-slot",
			"Put a card into a binder.", http.StatusCreated),
		func(ctx context.Context, req *addSlotInput) (*slotOutput, error) {
			ownerID, err := actor(ctx)
			if err != nil {
				return nil, errNoActor
			}

			slot, err := svc.AddSlot(ctx, ownerID, req.BinderID, model.AddSlotInput{
				SlotCard: req.Body.card().toSlotCard(),
				Position: req.Body.Position,
			})
			if err != nil {
				return nil, handleErr(ctx, fmt.Errorf("adding a card to the binder: %w", err))
			}
			return &slotOutput{Body: toSlotBody(slot)}, nil
		})

	huma.Register(api,
		newOp(http.MethodPatch, "/binders/{binderId}/slots", "move-binder-slot",
			"Move a card to another position, taking the cards between along.", http.StatusNoContent),
		func(ctx context.Context, req *moveSlotInput) (*struct{}, error) {
			ownerID, err := actor(ctx)
			if err != nil {
				return nil, errNoActor
			}

			err = svc.MoveSlot(ctx, ownerID, req.BinderID, model.MoveSlotInput{
				SlotID:     req.Body.SlotID,
				ToPosition: req.Body.ToPosition,
			})
			if err != nil {
				return nil, handleErr(ctx, fmt.Errorf("moving a card in the binder: %w", err))
			}
			return nil, nil //nolint:nilnil // 204: huma sends no body for an empty output struct.
		})

	huma.Register(api,
		newOp(http.MethodDelete, "/binders/{binderId}/slots/{slotId}", "remove-binder-slot",
			"Take a card out of a binder, closing the gap it leaves.", http.StatusNoContent),
		func(ctx context.Context, req *removeSlotInput) (*struct{}, error) {
			ownerID, err := actor(ctx)
			if err != nil {
				return nil, errNoActor
			}

			if err := svc.RemoveSlot(ctx, ownerID, req.BinderID, req.SlotID); err != nil {
				return nil, handleErr(ctx, fmt.Errorf("removing a card from the binder: %w", err))
			}
			return nil, nil //nolint:nilnil // 204: huma sends no body for an empty output struct.
		})
}

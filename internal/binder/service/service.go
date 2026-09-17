// Package service holds the binder domain's business logic: who may touch a
// binder, where a card may go in it, and the rule the database cannot state —
// that a binder's positions are dense.
package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/simeonkorchev/binder/internal/binder/model"
	"github.com/simeonkorchev/binder/internal/dataerror"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

var (
	// ErrBinderNotFound is returned for a binder that does not exist and for
	// one that is not the actor's. They are deliberately the same answer: a
	// distinct "forbidden" would confirm to a stranger that the binder exists.
	ErrBinderNotFound = errors.New("binder not found")
	// ErrSlotNotFound is returned for a slot that is not in the given binder.
	ErrSlotNotFound = errors.New("binder slot not found")
	// ErrPositionOutOfRange is returned when a position would leave a hole.
	// Positions are dense, so the only positions a card may be added at are
	// 0..count and the only ones it may be moved to are 0..count-1.
	ErrPositionOutOfRange = errors.New("position is outside the binder")
	// ErrResolutionUnknown is returned for a set resolution that is not a rung
	// of the match ladder.
	ErrResolutionUnknown = errors.New("set resolution is not a rung of the match ladder")
	// ErrPrintingRequired is returned when a resolution that determines a set
	// arrives without the printing it determined.
	ErrPrintingRequired = errors.New("this set resolution requires a card printing")
	// ErrPrintingNotAllowed is returned when a resolution that determines no
	// set arrives carrying a printing anyway.
	ErrPrintingNotAllowed = errors.New("this set resolution cannot carry a card printing")
	// ErrCardUnknown is returned when the card a slot names does not exist.
	ErrCardUnknown = errors.New("card does not exist")
	// ErrPrintingUnknown is returned when the printing a slot names does not
	// exist, or exists but is a printing of a different card.
	ErrPrintingUnknown = errors.New("card printing is not a printing of this card")
	// ErrPositionTaken is returned when a concurrent write took the position
	// this one was about to use. The request was correct when it was made, so
	// the answer is "try again", not "change something".
	ErrPositionTaken = errors.New("position was taken by a concurrent change")
	// ErrBatchTooLarge is returned for a batch over model.MaxSlotsPerBatch.
	// Nothing is written: the client splits the commit and sends it again.
	ErrBatchTooLarge = errors.New("too many cards in one batch")
)

// BinderStore reads and writes binders and reads their slots in bulk.
type BinderStore interface {
	// InTx runs cb inside a transaction, joining one already on ctx.
	InTx(ctx context.Context, cb func(ctx context.Context) error) error
	CreateBinder(ctx context.Context, binder model.Binder) (model.Binder, error)
	// GetBinderByID returns a dataerror.MissingEntityError when there is no
	// such binder.
	GetBinderByID(ctx context.Context, id uuid.UUID) (model.Binder, error)
	// ListBindersByOwner returns an empty slice for an owner with no binders.
	ListBindersByOwner(ctx context.Context, ownerID uuid.UUID) ([]model.Binder, error)
	UpdateBinderName(ctx context.Context, id uuid.UUID, name string) (model.Binder, error)
	CountSlots(ctx context.Context, binderID uuid.UUID) (int, error)
	// ListSlotsInRange returns the slots between from and to inclusive, in
	// position order, and an empty slice for a range with nothing in it.
	ListSlotsInRange(ctx context.Context, binderID uuid.UUID, from, to int) ([]model.Slot, error)
}

// SlotStore writes the slots of a binder and rearranges their positions. It is
// separate from BinderStore because the two are used by different halves of the
// service; one concrete store satisfies both.
type SlotStore interface {
	// GetSlotByID returns a dataerror.MissingEntityError when the slot is not
	// in that binder — including when it is in somebody else's.
	GetSlotByID(ctx context.Context, binderID, slotID uuid.UUID) (model.Slot, error)
	// InsertSlot requires the position to already be free.
	InsertSlot(ctx context.Context, slot model.Slot) (model.Slot, error)
	// InsertSlots writes every slot in one statement, so none of them is
	// written if any of them is refused, and returns them in position order.
	// Every position must already be free. An empty batch writes nothing and
	// returns an empty slice, which is not an error.
	InsertSlots(ctx context.Context, slots []model.Slot) ([]model.Slot, error)
	// DeleteSlotByID leaves a hole at the slot's position for
	// ClosePositionGap to close.
	DeleteSlotByID(ctx context.Context, binderID, slotID uuid.UUID) error
	OpenPositionGap(ctx context.Context, binderID uuid.UUID, at int) error
	ClosePositionGap(ctx context.Context, binderID uuid.UUID, at int) error
	MoveSlotPosition(ctx context.Context, binderID, slotID uuid.UUID, from, to int) error
}

// Store is the whole persistence surface of this domain, composed from the two
// role interfaces above so that one concrete store satisfies it and each half
// of the service still declares only what it calls.
//
//counterfeiter:generate . Store
type Store interface {
	BinderStore
	SlotStore
}

// Service is the binder domain's business logic.
type Service struct {
	store Store
}

// NewService returns a service over store.
func NewService(store Store) *Service {
	return &Service{store: store}
}

// requireBinderOwner refuses a binder to anyone but its owner. Every operation
// in this package starts here, so ownership is checked in one place and the
// store never has to know who is asking.
//
// A binder that is not there and one that is somebody else's are the same
// answer on purpose: a distinct "forbidden" would confirm to a stranger that
// the binder exists.
func (s *Service) requireBinderOwner(ctx context.Context, ownerID, binderID uuid.UUID) error {
	binder, err := s.store.GetBinderByID(ctx, binderID)
	if err != nil {
		if dataerror.IsMissingEntityError(err) {
			return ErrBinderNotFound
		}
		return fmt.Errorf("getting the binder: %w", err)
	}
	if binder.OwnerID != ownerID {
		return ErrBinderNotFound
	}
	return nil
}

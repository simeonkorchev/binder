package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/simeonkorchev/binder/internal/binder/model"
	cardmodel "github.com/simeonkorchev/binder/internal/card/model"
	"github.com/simeonkorchev/binder/internal/dataerror"
)

// AddSlot puts a card into a binder. A nil position appends it after the last
// card, which is what a scan does; a position inserts it there and shifts
// everything from there on one place along.
func (s *Service) AddSlot(
	ctx context.Context,
	ownerID uuid.UUID,
	binderID uuid.UUID,
	input model.AddSlotInput,
) (model.Slot, error) {
	// Checked before the transaction opens: these are facts about the request
	// itself, and none of them needs to read a row to decide.
	if err := validateResolution(input); err != nil {
		return model.Slot{}, err
	}

	var added model.Slot
	err := s.store.InTx(ctx, func(ctx context.Context) error {
		if err := s.requireBinderOwner(ctx, ownerID, binderID); err != nil {
			return err
		}

		count, err := s.store.CountSlots(ctx, binderID)
		if err != nil {
			return fmt.Errorf("counting the binder's slots: %w", err)
		}

		position, err := insertPosition(input.Position, count)
		if err != nil {
			return err
		}

		// Only a position with something at it needs a gap opened; appending
		// lands on a position that is already free.
		if position < count {
			if err := s.store.OpenPositionGap(ctx, binderID, position); err != nil {
				return fmt.Errorf("making room at position %d: %w", position, err)
			}
		}

		stored, err := s.store.InsertSlot(ctx, model.Slot{
			ID:             uuid.New(),
			BinderID:       binderID,
			Position:       position,
			CardID:         input.CardID,
			CardPrintingID: input.CardPrintingID,
			SetResolution:  input.SetResolution,
			// Zero on purpose: the timestamps come back from the database.
			CreatedAt: time.Time{},
			UpdatedAt: time.Time{},
		})
		if err != nil {
			return translateSlotWriteError(err)
		}
		added = stored
		return nil
	})
	if err != nil {
		return model.Slot{}, err
	}
	return added, nil
}

// MoveSlot moves one card to a different position and takes every card between
// its old and new position one place along, so the binder stays dense.
func (s *Service) MoveSlot(
	ctx context.Context,
	ownerID uuid.UUID,
	binderID uuid.UUID,
	input model.MoveSlotInput,
) error {
	return s.store.InTx(ctx, func(ctx context.Context) error {
		if err := s.requireBinderOwner(ctx, ownerID, binderID); err != nil {
			return err
		}

		slot, err := s.slotInBinder(ctx, binderID, input.SlotID)
		if err != nil {
			return err
		}

		count, err := s.store.CountSlots(ctx, binderID)
		if err != nil {
			return fmt.Errorf("counting the binder's slots: %w", err)
		}
		// A move cannot extend the binder, so the last legal destination is the
		// last occupied position.
		if input.ToPosition < 0 || input.ToPosition > count-1 {
			return ErrPositionOutOfRange
		}

		if slot.Position == input.ToPosition {
			return nil
		}

		if err := s.store.MoveSlotPosition(ctx, binderID, slot.ID, slot.Position, input.ToPosition); err != nil {
			return fmt.Errorf("moving the slot: %w", err)
		}
		return nil
	})
}

// RemoveSlot takes a card out of a binder and closes the hole it leaves. The
// two happen in one transaction because a binder with a hole in it is a state
// no reader may see: the database cannot enforce density, so nothing downstream
// would notice.
func (s *Service) RemoveSlot(ctx context.Context, ownerID, binderID, slotID uuid.UUID) error {
	return s.store.InTx(ctx, func(ctx context.Context) error {
		if err := s.requireBinderOwner(ctx, ownerID, binderID); err != nil {
			return err
		}

		slot, err := s.slotInBinder(ctx, binderID, slotID)
		if err != nil {
			return err
		}

		if err := s.store.DeleteSlotByID(ctx, binderID, slotID); err != nil {
			return fmt.Errorf("removing the slot: %w", err)
		}
		if err := s.store.ClosePositionGap(ctx, binderID, slot.Position); err != nil {
			return fmt.Errorf("closing the gap at position %d: %w", slot.Position, err)
		}
		return nil
	})
}

// slotInBinder fetches a slot of this binder, turning the store's missing
// entity into this domain's sentinel.
func (s *Service) slotInBinder(ctx context.Context, binderID, slotID uuid.UUID) (model.Slot, error) {
	slot, err := s.store.GetSlotByID(ctx, binderID, slotID)
	if err != nil {
		if dataerror.IsMissingEntityError(err) {
			return model.Slot{}, ErrSlotNotFound
		}
		return model.Slot{}, fmt.Errorf("getting the binder slot: %w", err)
	}
	return slot, nil
}

// validateResolution enforces the pairing the binder_slots CHECK states in SQL,
// before the INSERT rather than after it: a CHECK violation coming back from
// the driver is a 500 telling the client the server broke, when in fact their
// request was wrong (000-principles.md section 10).
func validateResolution(input model.AddSlotInput) error {
	if !cardmodel.ValidSetResolution(input.SetResolution) {
		return ErrResolutionUnknown
	}

	requiresPrinting := cardmodel.RequiresPrinting(input.SetResolution)
	if requiresPrinting && input.CardPrintingID == nil {
		return ErrPrintingRequired
	}
	if !requiresPrinting && input.CardPrintingID != nil {
		return ErrPrintingNotAllowed
	}
	return nil
}

// insertPosition decides where a card goes. Nil appends it; a value has to land
// inside the binder or immediately after its last card, because any other
// position would leave a hole.
func insertPosition(requested *int, count int) (int, error) {
	if requested == nil {
		return count, nil
	}
	if *requested < 0 || *requested > count {
		return 0, ErrPositionOutOfRange
	}
	return *requested, nil
}

// translateSlotWriteError turns the store's typed persistence errors into this
// domain's sentinels, so the api layer has one table to map and never sees a
// dataerror type.
func translateSlotWriteError(err error) error {
	switch {
	case dataerror.IsInvalidReferenceError(err, model.ReferenceCard):
		return ErrCardUnknown
	case dataerror.IsInvalidReferenceError(err, model.ReferenceCardPrinting):
		return ErrPrintingUnknown
	case dataerror.IsConflictError(err):
		return ErrPositionTaken
	default:
		return fmt.Errorf("adding the card to the binder: %w", err)
	}
}

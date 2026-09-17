package service

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/simeonkorchev/binder/internal/binder/model"
)

// AddSlots puts a whole reviewed scan session into a binder: every card or
// none, appended in order after the binder's last one.
//
// This is the operation the scan loop commits through, and the reason it exists
// at all is the transaction boundary. Adding sixty cards one request at a time
// is sixty transactions, and a dropped connection at card forty leaves a binder
// holding thirty-nine — after the user has physically put the rest away, so
// they cannot tell what landed without scanning the lot again. Here the
// ownership check, the count and the insert are one transaction: the binder
// either holds the whole sweep or is untouched.
//
// Positions stay dense by construction. The batch takes count..count+n-1, which
// the density invariant guarantees are free, so nothing has to be shifted and
// no row has to be parked above max(position) the way a mid-binder insert does.
func (s *Service) AddSlots(
	ctx context.Context,
	ownerID uuid.UUID,
	binderID uuid.UUID,
	input model.AddSlotsInput,
) ([]model.Slot, error) {
	// Checked before the transaction opens: these are facts about the request
	// itself, and none of them needs to read a row to decide. The whole batch
	// is checked before any of it is written, so a card the service was always
	// going to refuse does not cost a transaction.
	if len(input.Cards) > model.MaxSlotsPerBatch {
		return nil, ErrBatchTooLarge
	}
	for _, card := range input.Cards {
		if err := validateResolution(card); err != nil {
			return nil, err
		}
	}

	var added []model.Slot
	err := s.store.InTx(ctx, func(ctx context.Context) error {
		if err := s.requireBinderOwner(ctx, ownerID, binderID); err != nil {
			return err
		}

		// An empty batch still gets here: a commit of nothing is not an error
		// (000-principles.md section 8b), but it is not a way to find out
		// whether somebody else's binder exists either.
		count, err := s.store.CountSlots(ctx, binderID)
		if err != nil {
			return fmt.Errorf("counting the binder's slots: %w", err)
		}

		slots := make([]model.Slot, 0, len(input.Cards))
		for i, card := range input.Cards {
			slots = append(slots, newSlot(binderID, count+i, card))
		}

		stored, err := s.store.InsertSlots(ctx, slots)
		if err != nil {
			return translateSlotWriteError(err)
		}
		added = stored
		return nil
	})
	if err != nil {
		return nil, err
	}
	return added, nil
}

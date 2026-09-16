package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/simeonkorchev/binder/internal/binder/model"
)

// CreateBinder gives an owner a new, empty binder. The id is minted here: a
// client never supplies one.
func (s *Service) CreateBinder(ctx context.Context, ownerID uuid.UUID, name string) (model.Binder, error) {
	binder, err := s.store.CreateBinder(ctx, model.Binder{
		ID:      uuid.New(),
		OwnerID: ownerID,
		Name:    name,
		// Zero on purpose: the timestamps are the database's to assign, and
		// the store returns the row as stored rather than this guess at it.
		CreatedAt: time.Time{},
		UpdatedAt: time.Time{},
	})
	if err != nil {
		return model.Binder{}, fmt.Errorf("creating binder: %w", err)
	}
	return binder, nil
}

// ListBinders returns the owner's binders, oldest first. An owner with none
// gets an empty slice, which is not an error.
func (s *Service) ListBinders(ctx context.Context, ownerID uuid.UUID) ([]model.Binder, error) {
	binders, err := s.store.ListBindersByOwner(ctx, ownerID)
	if err != nil {
		return nil, fmt.Errorf("listing binders: %w", err)
	}
	return binders, nil
}

// RenameBinder renames one of the owner's binders.
func (s *Service) RenameBinder(
	ctx context.Context,
	ownerID uuid.UUID,
	binderID uuid.UUID,
	name string,
) (model.Binder, error) {
	var renamed model.Binder
	err := s.store.InTx(ctx, func(ctx context.Context) error {
		if err := s.requireBinderOwner(ctx, ownerID, binderID); err != nil {
			return err
		}

		updated, err := s.store.UpdateBinderName(ctx, binderID, name)
		if err != nil {
			return fmt.Errorf("updating the binder name: %w", err)
		}
		renamed = updated
		return nil
	})
	if err != nil {
		return model.Binder{}, err
	}
	return renamed, nil
}

// GetPage returns one 3x3 page of a binder: the slots actually on it and how
// many pages the binder has.
//
// A page past the end of the binder is an empty page, not an error — reading
// beyond a collection is an empty result (000-principles.md section 8b), and an
// empty binder has no pages at all, which is what PageCount(0) == 0 says.
func (s *Service) GetPage(
	ctx context.Context,
	ownerID uuid.UUID,
	binderID uuid.UUID,
	page int,
) (model.Page, error) {
	if page < 0 {
		return model.Page{}, ErrPositionOutOfRange
	}

	var result model.Page
	err := s.store.InTx(ctx, func(ctx context.Context) error {
		if err := s.requireBinderOwner(ctx, ownerID, binderID); err != nil {
			return err
		}

		count, err := s.store.CountSlots(ctx, binderID)
		if err != nil {
			return fmt.Errorf("counting the binder's slots: %w", err)
		}

		first := model.FirstPositionOnPage(page)
		slots, err := s.store.ListSlotsInRange(ctx, binderID, first, first+model.SlotsPerPage-1)
		if err != nil {
			return fmt.Errorf("listing the page's slots: %w", err)
		}

		result = model.Page{Number: page, Slots: slots, PageCount: model.PageCount(count)}
		return nil
	})
	if err != nil {
		return model.Page{}, err
	}
	return result, nil
}

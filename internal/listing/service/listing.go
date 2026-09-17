package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/simeonkorchev/binder/internal/dataerror"
	"github.com/simeonkorchev/binder/internal/listing/model"
)

// CreateListing marks one of the actor's own cards for sale.
//
// The ownership read and the insert are one transaction because they are one
// decision: the slot this actor was allowed to list is the slot that gets
// listed, even if the card is being removed from the binder at the same moment.
func (s *Service) CreateListing(ctx context.Context, ownerID, binderSlotID uuid.UUID) (model.Listing, error) {
	var created model.Listing
	err := s.store.InTx(ctx, func(ctx context.Context) error {
		if err := s.requireSlotOwner(ctx, ownerID, binderSlotID); err != nil {
			return err
		}

		stored, err := s.store.InsertListing(ctx, model.Listing{
			ID:           uuid.New(),
			BinderSlotID: binderSlotID,
			// Zero on purpose: the timestamps come back from the database.
			CreatedAt: time.Time{},
			UpdatedAt: time.Time{},
		})
		if err != nil {
			return translateListingWriteError(err)
		}
		created = stored
		return nil
	})
	if err != nil {
		return model.Listing{}, err
	}
	return created, nil
}

// DeleteListing takes one of the actor's own listings down. The card stays in
// the binder; only the offer to sell it goes away.
func (s *Service) DeleteListing(ctx context.Context, ownerID, listingID uuid.UUID) error {
	return s.store.InTx(ctx, func(ctx context.Context) error {
		if err := s.requireListingSeller(ctx, ownerID, listingID); err != nil {
			return err
		}

		if err := s.store.DeleteListingByID(ctx, listingID); err != nil {
			if dataerror.IsMissingEntityError(err) {
				return ErrListingNotFound
			}
			return fmt.Errorf("deleting the listing: %w", err)
		}
		return nil
	})
}

// BrowseListings returns the cards for sale that match search, newest first.
// Nothing matching is an empty slice, not an error.
//
// It takes no actor: the feed is what a buyer browses, and it carries no contact
// details — those are a separate request for one named seller.
func (s *Service) BrowseListings(ctx context.Context, search model.ListingSearch) ([]model.ListedCard, error) {
	listed, err := s.store.BrowseListings(ctx, search)
	if err != nil {
		return nil, fmt.Errorf("browsing listings: %w", err)
	}
	return listed, nil
}

// requireSlotOwner refuses a slot to anyone but the owner of the binder it is
// in, so that ownership is checked in one place and the store never has to know
// who is asking.
func (s *Service) requireSlotOwner(ctx context.Context, ownerID, binderSlotID uuid.UUID) error {
	seller, err := s.store.SlotSeller(ctx, binderSlotID)
	if err != nil {
		if dataerror.IsMissingEntityError(err) {
			return ErrBinderSlotNotFound
		}
		return fmt.Errorf("getting the seller of the binder slot: %w", err)
	}
	if seller != ownerID {
		return ErrBinderSlotNotFound
	}
	return nil
}

// requireListingSeller refuses a listing to anyone but the seller behind it.
func (s *Service) requireListingSeller(ctx context.Context, ownerID, listingID uuid.UUID) error {
	seller, err := s.store.ListingSeller(ctx, listingID)
	if err != nil {
		if dataerror.IsMissingEntityError(err) {
			return ErrListingNotFound
		}
		return fmt.Errorf("getting the seller of the listing: %w", err)
	}
	if seller != ownerID {
		return ErrListingNotFound
	}
	return nil
}

// translateListingWriteError turns the store's typed persistence errors into
// this domain's sentinels, so the api layer has one table to map and never sees
// a dataerror type.
//
// An unusable slot reference here is not a bad request: the slot was there when
// its owner was read a moment ago, so the card has just been taken out of the
// binder. "It is no longer there" is the honest answer, and it is the same one
// the read would have given had it run a moment later.
func translateListingWriteError(err error) error {
	switch {
	case dataerror.IsConflictError(err):
		return ErrAlreadyListed
	case dataerror.IsInvalidReferenceError(err, model.ReferenceBinderSlot):
		return ErrBinderSlotNotFound
	default:
		return fmt.Errorf("marking the card for sale: %w", err)
	}
}

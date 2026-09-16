package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
	"github.com/simeonkorchev/binder/internal/binder/model"
)

// BinderService is the half of the domain that owns binders themselves.
//
// It is faked through api.Service, which embeds it.
type BinderService interface {
	CreateBinder(ctx context.Context, ownerID uuid.UUID, name string) (model.Binder, error)
	ListBinders(ctx context.Context, ownerID uuid.UUID) ([]model.Binder, error)
	RenameBinder(ctx context.Context, ownerID, binderID uuid.UUID, name string) (model.Binder, error)
	GetPage(ctx context.Context, ownerID, binderID uuid.UUID, page int) (model.Page, error)
}

// binderBody is a binder on the wire.
type binderBody struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	// OwnerID is deliberately not exposed: every binder a client can see is
	// its own, so the field would carry no information and would say who owns
	// a binder to anyone who found an id.
}

func toBinderBody(binder model.Binder) binderBody {
	return binderBody{
		ID:        binder.ID,
		Name:      binder.Name,
		CreatedAt: binder.CreatedAt,
		UpdatedAt: binder.UpdatedAt,
	}
}

type createBinderInput struct {
	Body struct {
		Name string `json:"name" minLength:"1" maxLength:"120" doc:"What to call the binder."`
	}
}

type binderOutput struct {
	Body binderBody
}

type listBindersOutput struct {
	Body struct {
		// Binders is empty, never null: an owner with no binders yet is the
		// ordinary first-run state, not an error.
		Binders []binderBody `json:"binders"`
	}
}

type renameBinderInput struct {
	BinderID uuid.UUID `path:"binderId"`
	Body     struct {
		Name string `json:"name" minLength:"1" maxLength:"120" doc:"What to call the binder."`
	}
}

type getPageInput struct {
	BinderID uuid.UUID `path:"binderId"`
	Page     int       `query:"page" required:"false" default:"0" minimum:"0" doc:"Zero-based page number."`
}

type getPageOutput struct {
	Body pageBody
}

// pageBody is one 3x3 page. Slots always has SlotsPerPage entries, with null
// for an empty pocket: the grid is how the page is drawn, and laying it out
// here means every client does not have to.
type pageBody struct {
	Page      int         `json:"page"`
	PageCount int         `json:"pageCount"`
	Slots     []*slotBody `json:"slots"`
}

func registerBinderEndpoints(api huma.API, svc BinderService, actor ActorFunc) {
	registerCreateBinder(api, svc, actor)
	registerListBinders(api, svc, actor)
	registerRenameBinder(api, svc, actor)
	registerGetBinderPage(api, svc, actor)
}

func registerCreateBinder(api huma.API, svc BinderService, actor ActorFunc) {
	huma.Register(api,
		newOp(http.MethodPost, "/binders", "create-binder", "Create a binder.", http.StatusCreated),
		func(ctx context.Context, req *createBinderInput) (*binderOutput, error) {
			ownerID, err := actor(ctx)
			if err != nil {
				return nil, errNoActor
			}

			binder, err := svc.CreateBinder(ctx, ownerID, req.Body.Name)
			if err != nil {
				return nil, handleErr(ctx, fmt.Errorf("creating binder: %w", err))
			}
			return &binderOutput{Body: toBinderBody(binder)}, nil
		})
}

func registerListBinders(api huma.API, svc BinderService, actor ActorFunc) {
	huma.Register(api,
		newOp(http.MethodGet, "/binders", "list-binders", "List the signed-in user's binders.", http.StatusOK),
		func(ctx context.Context, _ *struct{}) (*listBindersOutput, error) {
			ownerID, err := actor(ctx)
			if err != nil {
				return nil, errNoActor
			}

			binders, err := svc.ListBinders(ctx, ownerID)
			if err != nil {
				return nil, handleErr(ctx, fmt.Errorf("listing binders: %w", err))
			}

			out := &listBindersOutput{}
			out.Body.Binders = toBinderBodies(binders)
			return out, nil
		})
}

func registerRenameBinder(api huma.API, svc BinderService, actor ActorFunc) {
	huma.Register(api,
		newOp(http.MethodPatch, "/binders/{binderId}", "rename-binder", "Rename a binder.", http.StatusOK),
		func(ctx context.Context, req *renameBinderInput) (*binderOutput, error) {
			ownerID, err := actor(ctx)
			if err != nil {
				return nil, errNoActor
			}

			binder, err := svc.RenameBinder(ctx, ownerID, req.BinderID, req.Body.Name)
			if err != nil {
				return nil, handleErr(ctx, fmt.Errorf("renaming binder: %w", err))
			}
			return &binderOutput{Body: toBinderBody(binder)}, nil
		})
}

func registerGetBinderPage(api huma.API, svc BinderService, actor ActorFunc) {
	huma.Register(api,
		newOp(http.MethodGet, "/binders/{binderId}", "get-binder-page",
			"Read one 3x3 page of a binder.", http.StatusOK),
		func(ctx context.Context, req *getPageInput) (*getPageOutput, error) {
			ownerID, err := actor(ctx)
			if err != nil {
				return nil, errNoActor
			}

			page, err := svc.GetPage(ctx, ownerID, req.BinderID, req.Page)
			if err != nil {
				return nil, handleErr(ctx, fmt.Errorf("reading a binder page: %w", err))
			}
			return &getPageOutput{Body: toPageBody(page)}, nil
		})
}

func toBinderBodies(binders []model.Binder) []binderBody {
	mapped := make([]binderBody, 0, len(binders))
	for _, binder := range binders {
		mapped = append(mapped, toBinderBody(binder))
	}
	return mapped
}

// toPageBody lays a page's occupied positions out into the fixed 3x3 grid the
// page is drawn as. A pocket with nothing in it is null, and a binder with no
// cards is a grid of nulls with a page count of zero.
func toPageBody(page model.Page) pageBody {
	grid := make([]*slotBody, model.SlotsPerPage)
	for _, slot := range page.Slots {
		mapped := toSlotBody(slot)
		grid[slot.SlotOnPage()] = &mapped
	}

	return pageBody{
		Page:      page.Number,
		PageCount: page.PageCount,
		Slots:     grid,
	}
}

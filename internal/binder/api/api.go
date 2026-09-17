// Package api is the binder domain's HTTP layer: binder CRUD, the paged 3x3
// read, and adding, moving and removing cards.
//
// Handlers here are thin. They resolve the actor, map the request to the
// service's input, call one service method, and map what comes back to a DTO of
// this package's own — the domain model is never serialised directly.
package api

import (
	"context"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

// binderTag groups this domain's operations in the generated OpenAPI document.
const binderTag = "binders"

// ActorFunc names the signed-in user behind a request. Identity comes from the
// server, never from the client (002-go-conventions.md section 8b), and which
// server — what the session token is and how it is verified — belongs to the
// user domain. This is the seam between the two: main.go passes the user
// domain's implementation, and a handler spec passes one that answers a fixed
// id. An error means there is no usable actor, which is a 401.
type ActorFunc func(ctx context.Context) (uuid.UUID, error)

// Service is everything the binder domain's handlers consume. It is composed by
// embedding the per-concern interfaces each handler file declares beside its
// own handlers, so one concrete service satisfies every one of them while no
// handler can reach a method it has no business calling. Only the composition
// is faked: a handler spec drives the whole registered API.
//
//counterfeiter:generate . Service
type Service interface {
	BinderService
	SlotService
}

// RegisterEndpoints mounts the binder domain's operations on api, resolving the
// actor of every request through actor.
func RegisterEndpoints(api huma.API, svc Service, actor ActorFunc) {
	registerBinderEndpoints(api, svc, actor)
	registerSlotEndpoints(api, svc, actor)
	registerBatchEndpoints(api, svc, actor)
}

// newOp builds one operation of this domain. Every operation goes through it so
// that the tag, and anything else the whole domain shares, is stated once.
func newOp(method, path, operationID, summary string, status int) huma.Operation {
	return huma.Operation{
		OperationID:   operationID,
		Method:        method,
		Path:          path,
		Summary:       summary,
		Tags:          []string{binderTag},
		DefaultStatus: status,
	}
}

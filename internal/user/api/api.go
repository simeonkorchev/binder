// Package api is the user domain's HTTP layer: exchanging a provider's identity
// token for a session, and reading and replacing the contact details a user
// publishes.
//
// It is also where every other domain gets its actor. Middleware resolves a
// session token to a user id and puts it on the request context; ActorFromContext
// reads it back, and is the binder and listing domains' ActorFunc. The context
// key stays in this package — identity is this domain's business, and a key
// copied into another domain is two places that have to agree
// (002-go-conventions.md section 3a: a cross-domain dependency is a consumer-side
// interface, never a reach into another domain's package).
//
// Handlers here are thin: resolve the actor, map the request to the service's
// input, call one service method, map what comes back to a DTO of this package's
// own. No handler chooses a status — see errors.go.
package api

import (
	"context"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
	"github.com/simeonkorchev/binder/internal/user/model"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

// userTag groups this domain's operations in the generated OpenAPI document.
const userTag = "account"

// Service is everything the user domain's handlers consume, composed from the
// per-concern interfaces each handler file declares beside its own handlers.
//
// SellerContact is deliberately absent: it exists for the listing domain, which
// declares its own interface over it, and no endpoint here serves it.
//
//counterfeiter:generate . Service
type Service interface {
	SessionService
	AccountService
}

// RegisterEndpoints mounts the user domain's operations on api, resolving the
// actor of every authenticated request through actor.
//
// actor is a parameter rather than ActorFromContext directly for the same reason
// the other domains take one: a handler spec passes one that answers a fixed id,
// so the handlers are covered without minting a real session for every request.
// main.go passes ActorFromContext.
func RegisterEndpoints(api huma.API, svc Service, actor ActorFunc) {
	registerSessionEndpoints(api, svc)
	registerAccountEndpoints(api, svc, actor)
}

// ActorFunc names the signed-in user behind a request. It is the same shape the
// binder and listing domains take, so one implementation serves all three; an
// error means there is no usable actor, which is a 401.
type ActorFunc func(ctx context.Context) (uuid.UUID, error)

// newOp builds one operation of this domain, so the tag and anything else the
// whole domain shares is stated once.
func newOp(method, path, operationID, summary string, status int) huma.Operation {
	return huma.Operation{
		OperationID:   operationID,
		Method:        method,
		Path:          path,
		Summary:       summary,
		Tags:          []string{userTag},
		DefaultStatus: status,
	}
}

// authProvider is model.AuthProvider on the wire. It is its own type so the
// OpenAPI enum is generated from model.AuthProviders() rather than re-typed into
// a struct tag that a third provider would not update.
type authProvider model.AuthProvider

// Schema declares the enum from the one list of providers.
func (authProvider) Schema(huma.Registry) *huma.Schema {
	providers := model.AuthProviders()
	values := make([]any, 0, len(providers))
	for _, provider := range providers {
		values = append(values, string(provider))
	}

	return &huma.Schema{
		Type:        huma.TypeString,
		Enum:        values,
		Description: "Which provider vouched for this account.",
	}
}

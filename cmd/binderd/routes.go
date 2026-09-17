package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	binderapi "github.com/simeonkorchev/binder/internal/binder/api"
	cardapi "github.com/simeonkorchev/binder/internal/card/api"
	listingapi "github.com/simeonkorchev/binder/internal/listing/api"
	userapi "github.com/simeonkorchev/binder/internal/user/api"
)

const (
	// apiTitle and apiVersion name the document every generated client is built
	// from. The version is the contract's, not the binary's: it changes when the
	// shape of an endpoint changes, which is not the same event as a deploy.
	apiTitle   = "Binder API"
	apiVersion = "1.0.0"
	// healthTag groups the liveness probe away from the domains in the document.
	healthTag = "health"
	// healthStatus is the only body /health ever returns.
	healthStatus = "ok"
	// specIndent is how the OpenAPI document is indented on disk. It is written
	// to a file that is diffed by the gate, so it is formatted for review.
	specIndent = "  "
)

// services is the set of domain services the registered operations call.
//
// The zero value is legitimate, and is exactly what the OpenAPI generator
// passes: registering an operation stores a handler closure, it never calls the
// service behind it, so the document is a function of the routes alone and needs
// no database. A nil field would panic the first time a request reached its
// handler — which cannot happen from the generator, because it serves nothing
// and exits.
type services struct {
	cards    cardapi.Service
	binders  binderapi.Service
	listings listingapi.Service
	accounts userapi.Service
}

// register mounts every domain's operations, and the liveness probe, on api.
//
// userapi.ActorFromContext is passed to all three domains that take an
// ActorFunc, unconverted: it is an ordinary function, so it is assignable to
// each domain's own named type, and internal/user/api/actor_test.go asserts at
// compile time that the three signatures have not drifted apart.
func register(api huma.API, svc services) {
	cardapi.RegisterEndpoints(api, svc.cards)
	binderapi.RegisterEndpoints(api, svc.binders, userapi.ActorFromContext)
	listingapi.RegisterEndpoints(api, svc.listings, userapi.ActorFromContext)
	userapi.RegisterEndpoints(api, svc.accounts, userapi.ActorFromContext)
	registerHealth(api)
}

// newHandler builds the whole HTTP surface: every domain's operations, wrapped
// in the session middleware that names the actor behind a request.
//
// The middleware authenticates and does not authorise. A request with no usable
// session reaches its handler with no actor and is answered 401 there, so the
// public endpoints — signing in, and browsing the marketplace — need no list of
// exempt paths here (internal/user/api/actor.go).
func newHandler(svc services, verifier userapi.SessionVerifier) http.Handler {
	mux := http.NewServeMux()
	register(humago.New(mux, huma.DefaultConfig(apiTitle, apiVersion)), svc)
	return userapi.Middleware(verifier)(mux)
}

// openAPIDocument renders the OpenAPI document for every operation newHandler
// registers, without a database, a listener or a line of configuration.
//
// It is the same register call the server makes, on the same config, so the
// document cannot describe an API this binary does not serve.
func openAPIDocument() ([]byte, error) {
	api := humago.New(http.NewServeMux(), huma.DefaultConfig(apiTitle, apiVersion))
	register(api, services{})

	document, err := json.MarshalIndent(api.OpenAPI(), "", specIndent)
	if err != nil {
		return nil, fmt.Errorf("rendering the OpenAPI document: %w", err)
	}
	return document, nil
}

// healthOutput is what the liveness probe answers.
type healthOutput struct {
	Body struct {
		Status string `json:"status" doc:"Always \"ok\"; the probe answering at all is the signal." example:"ok"`
	}
}

// registerHealth mounts the liveness probe.
//
// It is liveness, not readiness: it reports that this process is up and routing,
// and deliberately does not touch the database. A probe that failed on a
// database blip would take a healthy instance out of rotation for a dependency
// it does not own, and every endpoint that needs the database already reports
// its own failure.
func registerHealth(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID:   "health",
		Method:        http.MethodGet,
		Path:          "/health",
		Summary:       "Report that the server is up.",
		Tags:          []string{healthTag},
		DefaultStatus: http.StatusOK,
	}, func(context.Context, *struct{}) (*healthOutput, error) {
		out := &healthOutput{}
		out.Body.Status = healthStatus
		return out, nil
	})
}

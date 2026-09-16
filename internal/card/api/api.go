// Package api is the card domain's HTTP layer: the scan-resolution endpoint the
// scanner calls once per captured card, and the card search behind it.
//
// Handlers here are thin. They map a request to the service's input, call one
// service method, and map what comes back to a DTO of this package's own — the
// domain model is never serialised directly, so a model change cannot silently
// change the wire format.
package api

import "github.com/danielgtaylor/huma/v2"

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate

// cardTag groups this domain's operations in the generated OpenAPI document.
const cardTag = "cards"

// Service is everything the card domain's handlers consume. It is composed by
// embedding the per-concern interfaces each handler file declares beside its
// own handlers, so one concrete service satisfies every one of them while no
// handler can reach a method it has no business calling. Only the composition
// is faked: a handler spec drives the whole registered API, so it needs one
// value that answers every operation.
//
//counterfeiter:generate . Service
type Service interface {
	ScanService
	CardService
}

// RegisterEndpoints mounts the card domain's operations on api.
func RegisterEndpoints(api huma.API, svc Service) {
	registerScanEndpoints(api, svc)
	registerCardEndpoints(api, svc)
}

// newOp builds one operation of this domain. Every operation goes through it so
// that the tag, and anything else the whole domain shares, is stated once.
func newOp(method, path, operationID, summary string, status int) huma.Operation {
	return huma.Operation{
		OperationID:   operationID,
		Method:        method,
		Path:          path,
		Summary:       summary,
		Tags:          []string{cardTag},
		DefaultStatus: status,
	}
}

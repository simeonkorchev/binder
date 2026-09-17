package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/simeonkorchev/binder/internal/listing/api"
	"github.com/simeonkorchev/binder/pkg/humaschema"
)

var (
	// errService is the don't-care service failure: the specs using it assert
	// the status a client gets, never this error's text.
	errService = errors.New("service failed")
	// errNoSession is what an ActorFunc returns for a request with no usable
	// session.
	errNoSession = errors.New("no session")
)

func TestAPI(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Listing API Suite")
}

// signedInAs is the ActorFunc a spec uses when the request is authenticated.
func signedInAs(userID uuid.UUID) api.ActorFunc {
	return func(context.Context) (uuid.UUID, error) { return userID, nil }
}

// signedOut is the ActorFunc for a request with no usable session.
func signedOut() api.ActorFunc {
	return func(context.Context) (uuid.UUID, error) { return uuid.Nil, errNoSession }
}

// newTestHandler registers the domain's real operations on a real Huma API, so
// a spec exercises routing, validation, the Resolve hooks and serialisation
// rather than calling a handler function directly.
func newTestHandler(svc api.Service, actor api.ActorFunc) http.Handler {
	mux := http.NewServeMux()
	api.RegisterEndpoints(humago.New(mux, humaschema.Config("Binder", "test")), svc, actor)
	return mux
}

// do runs one request through the registered API. body is marshalled when it is
// not nil.
func do(svc api.Service, actor api.ActorFunc, method, target string, body any) *httptest.ResponseRecorder {
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		encoded, err := json.Marshal(body)
		Expect(err).NotTo(HaveOccurred())
		reader = bytes.NewReader(encoded)
	}

	req := httptest.NewRequestWithContext(context.Background(), method, target, reader)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	newTestHandler(svc, actor).ServeHTTP(rec, req)
	return rec
}

// decodeBody reads a response body into target, failing the spec rather than
// the caller when it is not the JSON the endpoint promised.
func decodeBody(rec *httptest.ResponseRecorder, target any) {
	Expect(json.Unmarshal(rec.Body.Bytes(), target)).To(Succeed(), "body was %s", rec.Body.String())
}

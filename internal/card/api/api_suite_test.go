package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/simeonkorchev/binder/internal/card/api"
)

// errService is the don't-care service failure: the specs using it assert the
// status a client gets, never this error's text.
var errService = errors.New("service failed")

func TestAPI(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Card API Suite")
}

// newTestHandler registers the domain's real operations on a real Huma API, so
// a spec exercises routing, validation, the Resolve hooks and serialisation
// rather than calling a handler function directly.
func newTestHandler(svc api.Service) http.Handler {
	mux := http.NewServeMux()
	api.RegisterEndpoints(humago.New(mux, huma.DefaultConfig("Binder", "test")), svc)
	return mux
}

// do runs one request through the registered API. body is marshalled when it is
// not nil, which is what tells a POST from a GET here.
func do(svc api.Service, method, target string, body any) *httptest.ResponseRecorder {
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
	newTestHandler(svc).ServeHTTP(rec, req)
	return rec
}

// decodeBody reads a response body into target, failing the spec rather than
// the caller when it is not the JSON the endpoint promised.
func decodeBody(rec *httptest.ResponseRecorder, target any) {
	Expect(json.Unmarshal(rec.Body.Bytes(), target)).To(Succeed(), "body was %s", rec.Body.String())
}

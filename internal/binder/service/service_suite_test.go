package service_test

import (
	"context"
	"errors"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/simeonkorchev/binder/internal/binder/service/servicefakes"
)

// errDB is the don't-care store failure: the specs using it assert how the
// service reacts to a failing store, never on this error's text.
var errDB = errors.New("db error")

func TestService(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Binder Service Suite")
}

// passThroughTx neutralises the transaction boundary so a spec exercises the
// closure's body rather than the store's transaction handling.
func passThroughTx(fake *servicefakes.FakeStore) {
	fake.InTxStub = func(ctx context.Context, cb func(context.Context) error) error {
		return cb(ctx)
	}
}

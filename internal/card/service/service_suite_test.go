package service_test

import (
	"errors"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// errDB is the don't-care store failure: the specs that use it assert how the
// service reacts to a failing store, never on this error's text.
var errDB = errors.New("db error")

func TestService(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Card Service Suite")
}

package service_test

import (
	"errors"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	// errDB is the don't-care store failure: the specs using it assert that the
	// failure is reported, never this error's text.
	errDB = errors.New("the database is unreachable")
	// errProviderDown is the don't-care verifier failure — an outage reaching the
	// provider's keys, as opposed to a token the provider would not have issued.
	errProviderDown = errors.New("the provider's keys could not be fetched")
	// errSigning is the don't-care failure to mint a session token.
	errSigning = errors.New("the session could not be signed")
)

func TestService(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "User Service Suite")
}

package identity_test

import (
	"context"
	"errors"
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/simeonkorchev/binder/pkg/oidc"
)

// errProviderDown is the don't-care verifier failure.
var errProviderDown = errors.New("the provider is unreachable")

func TestIdentity(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "User Identity Suite")
}

// stubVerifier stands in for one provider's oidc.Verifier: it records the token
// and nonce it was handed and answers with whatever the spec set.
type stubVerifier struct {
	identity oidc.Identity
	err      error

	calls         int
	gotToken      string
	gotNonce      string
	gotContextNil bool
}

func (s *stubVerifier) Verify(ctx context.Context, rawToken, expectedNonce string) (oidc.Identity, error) {
	s.calls++
	s.gotToken = rawToken
	s.gotNonce = expectedNonce
	s.gotContextNil = ctx == nil
	return s.identity, s.err
}

// withoutEnv unsets a variable for the duration of one spec and puts back
// whatever was there, so a spec about a missing variable is not at the mercy of
// the ambient environment.
func withoutEnv(name string) {
	previous, wasSet := os.LookupEnv(name)
	Expect(os.Unsetenv(name)).To(Succeed())
	DeferCleanup(func() {
		if wasSet {
			Expect(os.Setenv(name, previous)).To(Succeed())
		}
	})
}

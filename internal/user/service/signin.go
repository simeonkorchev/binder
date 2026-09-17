package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/simeonkorchev/binder/internal/user/model"
	"github.com/simeonkorchev/binder/pkg/oidc"
)

// SignIn exchanges a provider's identity token for a session on this backend,
// creating the account if this is the first time this provider identity has been
// seen.
//
// Nothing about the caller is trusted: the provider's signature over its own
// claims is what makes the subject an identity, and a token that does not verify
// is ErrIdentityRejected without an account being created, read or touched.
func (s *Service) SignIn(ctx context.Context, input model.SignIn) (model.Session, error) {
	// Checked here as well as at the API boundary. The boundary's enum gives the
	// client a 422 naming the field; this is what keeps a caller that is not an
	// HTTP request from reaching a verifier map that has no entry for it
	// (000-principles.md section 10).
	if !model.ValidAuthProvider(input.Provider) {
		return model.Session{}, fmt.Errorf("%w: %q", ErrProviderUnknown, input.Provider)
	}

	identity, err := s.verifier.Verify(ctx, input.Provider, input.IdentityToken, input.Nonce)
	if err != nil {
		if errors.Is(err, oidc.ErrTokenRejected) {
			// Joined with its cause so this layer's caller can match the
			// sentinel while the cause is still there for a 500 that needs it.
			// The cause carries no token material: pkg/oidc puts no claim and no
			// token bytes in its errors.
			return model.Session{}, fmt.Errorf("%w: %w", ErrIdentityRejected, err)
		}
		// Reaching the provider's keys failed. That is our outage, not a bad
		// token, and it must not become a 401.
		return model.Session{}, fmt.Errorf("verifying the identity token: %w", err)
	}

	user, err := s.store.EnsureUserByIdentity(ctx, s.newUserID(), identity)
	if err != nil {
		return model.Session{}, fmt.Errorf("resolving the account behind a provider identity: %w", err)
	}

	token, err := s.sessions.Issue(user.ID)
	if err != nil {
		return model.Session{}, fmt.Errorf("issuing the session: %w", err)
	}

	// The one thing logged about a sign-in, and deliberately all of it: which
	// account, and which provider vouched for it. The identity token, the
	// provider's subject and every contact detail stay out of the logs — this
	// domain handles nothing but personal data, so the safe default is to log an
	// id and nothing else.
	slog.InfoContext(ctx, "user signed in",
		slog.String("user_id", user.ID.String()),
		slog.String("auth_provider", string(identity.Provider)),
	)

	return model.Session{Token: token.Value, ExpiresAt: token.ExpiresAt, User: user}, nil
}

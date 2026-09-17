package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/simeonkorchev/binder/internal/user/model"
)

// SessionService is the half of the domain that turns a provider's identity token
// into a session here.
//
// It is faked through api.Service, which embeds it.
type SessionService interface {
	SignIn(ctx context.Context, input model.SignIn) (model.Session, error)
}

// signInBody is a sign-in as the app sends it.
type signInBody struct {
	Provider authProvider `json:"provider" doc:"Which provider the identity token came from."`
	// IdentityToken is the provider's ID token, unchanged. It is verified against
	// the provider's published keys on this side: nothing in it is trusted
	// because the app sent it.
	IdentityToken string `json:"identityToken" minLength:"1" maxLength:"8192" doc:"The provider's identity token."`
	// Nonce is what the app asked the provider to bind this token to. Leave it
	// out only if the app sent no nonce to the provider — a token that carries
	// one is refused without it, because accepting it would throw away the
	// binding that stops the token being replayed.
	Nonce string `json:"nonce" required:"false" maxLength:"512" doc:"The nonce this sign-in was bound to."`
}

// Resolve refuses a provider the enum does not cover before the service is
// reached, so the client is told which field is wrong. The service checks the
// same thing again: this guard is not a licence for the layer below to assume
// (000-principles.md section 10).
func (b signInBody) Resolve(huma.Context) []error {
	if !model.ValidAuthProvider(model.AuthProvider(b.Provider)) {
		return []error{&huma.ErrorDetail{
			Message:  "provider has to be one this app signs in with.",
			Location: "body.provider",
			Value:    string(b.Provider),
		}}
	}
	return nil
}

type signInInput struct {
	Body signInBody
}

// sessionBody is a session on the wire: the token to send back on every later
// request, when it stops working, and the account it belongs to.
type sessionBody struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expiresAt"`
	User      userBody  `json:"user"`
}

type sessionOutput struct {
	Body sessionBody
}

func toSessionBody(session model.Session) sessionBody {
	return sessionBody{
		Token:     session.Token,
		ExpiresAt: session.ExpiresAt,
		User:      toUserBody(session.User),
	}
}

func registerSessionEndpoints(api huma.API, svc SessionService) {
	huma.Register(api,
		// No actor: this is the one endpoint that runs before there is one.
		newOp(http.MethodPost, "/auth/sessions", "sign-in",
			"Exchange a provider's identity token for a session.", http.StatusCreated),
		func(ctx context.Context, req *signInInput) (*sessionOutput, error) {
			session, err := svc.SignIn(ctx, model.SignIn{
				Provider:      model.AuthProvider(req.Body.Provider),
				IdentityToken: req.Body.IdentityToken,
				Nonce:         req.Body.Nonce,
			})
			if err != nil {
				return nil, handleErr(ctx, fmt.Errorf("signing in: %w", err))
			}
			return &sessionOutput{Body: toSessionBody(session)}, nil
		})
}

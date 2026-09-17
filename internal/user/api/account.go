package api

import (
	"context"
	"fmt"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
	"github.com/simeonkorchev/binder/internal/user/model"
)

// AccountService is the half of the domain that owns what an account publishes
// about itself.
//
// It is faked through api.Service, which embeds it.
type AccountService interface {
	GetUser(ctx context.Context, userID uuid.UUID) (model.User, error)
	SetContact(ctx context.Context, userID uuid.UUID, contact model.Contact) (model.User, error)
}

// userBody is an account on the wire.
//
// Three fields of model.User are deliberately not carried across
// (000-principles.md section 9):
//   - Identity, because the provider's subject is the account's name at the
//     provider and no client has any use for it — the app already knows which
//     provider it signed in with, because it did it;
//   - CreatedAt and UpdatedAt, because nothing in the app shows either, and a
//     timestamp nobody reads is a field that can only ever disagree with the row.
type userBody struct {
	ID uuid.UUID `json:"id"`
	// ContactEmail and ContactPhone are null when the account has not opted into
	// sharing them, which is the normal state and not a missing value.
	ContactEmail *string `json:"contactEmail"`
	ContactPhone *string `json:"contactPhone"`
}

func toUserBody(user model.User) userBody {
	return userBody{
		ID:           user.ID,
		ContactEmail: user.Contact.Email,
		ContactPhone: user.Contact.Phone,
	}
}

type userOutput struct {
	Body userBody
}

// setContactBody replaces the whole contact preference rather than patching part
// of it: a field left out or sent as null is "not shared". PUT rather than PATCH
// for exactly that reason — with a patch there would be no way to say "stop
// sharing my phone number", because absent and null would have to mean both
// "leave it alone" and "clear it".
type setContactBody struct {
	Email *string `json:"email" required:"false" maxLength:"320" doc:"The email address to publish, or null."`
	Phone *string `json:"phone" required:"false" maxLength:"32" doc:"The phone number to publish, or null."`
}

// Resolve refuses a field that is present but blank, naming it, so the client is
// told which one is wrong instead of getting a bare 422. The service refuses it
// again, and the column refuses it after that.
func (b setContactBody) Resolve(huma.Context) []error {
	_, blank := b.contact().Normalise()

	details := make([]error, 0, len(blank))
	for _, field := range blank {
		details = append(details, &huma.ErrorDetail{
			Message:  "A contact detail you share cannot be blank. Leave it out to stop sharing it.",
			Location: "body." + string(field),
			// The value is deliberately not echoed: it is whitespace, so there is
			// nothing to show, and an error body is a thing that gets logged.
			Value: nil,
		})
	}
	if len(details) == 0 {
		return nil
	}
	return details
}

// contact is the request as the domain sees it.
func (b setContactBody) contact() model.Contact {
	return model.Contact{Email: b.Email, Phone: b.Phone}
}

type setContactInput struct {
	Body setContactBody
}

func registerAccountEndpoints(api huma.API, svc AccountService, actor ActorFunc) {
	registerGetAccount(api, svc, actor)
	registerSetContact(api, svc, actor)
}

func registerGetAccount(api huma.API, svc AccountService, actor ActorFunc) {
	huma.Register(api,
		newOp(http.MethodGet, "/me", "get-account", "Read the signed-in account.", http.StatusOK),
		func(ctx context.Context, _ *struct{}) (*userOutput, error) {
			// The account read is always the caller's own: there is no path
			// parameter to name somebody else's, so there is nothing to authorise
			// beyond having a session.
			userID, err := actor(ctx)
			if err != nil {
				return nil, errNoActor
			}

			user, err := svc.GetUser(ctx, userID)
			if err != nil {
				return nil, handleErr(ctx, fmt.Errorf("reading the account: %w", err))
			}
			return &userOutput{Body: toUserBody(user)}, nil
		})
}

func registerSetContact(api huma.API, svc AccountService, actor ActorFunc) {
	huma.Register(api,
		newOp(http.MethodPut, "/me/contact", "set-account-contact",
			"Replace the contact details this account publishes.", http.StatusOK),
		func(ctx context.Context, req *setContactInput) (*userOutput, error) {
			userID, err := actor(ctx)
			if err != nil {
				return nil, errNoActor
			}

			user, err := svc.SetContact(ctx, userID, req.Body.contact())
			if err != nil {
				return nil, handleErr(ctx, fmt.Errorf("replacing the account's contact details: %w", err))
			}
			return &userOutput{Body: toUserBody(user)}, nil
		})
}

package api

import (
	"context"
	"fmt"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/google/uuid"
	"github.com/simeonkorchev/binder/internal/listing/model"
)

// SellerService is the half of the domain that answers "how do I reach this
// seller".
//
// It is faked through api.Service, which embeds it.
type SellerService interface {
	SellerContact(ctx context.Context, sellerID uuid.UUID) (model.SellerContact, error)
}

type sellerContactInput struct {
	SellerID uuid.UUID `path:"sellerId"`
}

type sellerContactOutput struct {
	Body sellerContactBody
}

// sellerContactBody is how to reach a seller: what they opted into publishing,
// and nothing else about them.
//
// Both fields null is a successful answer and the normal state of an account
// that opted into neither — the client shows no contact button for it. It is
// deliberately not a 404 and not an error (D4, 000-principles.md section 8b),
// and there is deliberately no "hasContact" flag beside the two fields: a second
// statement of the same fact is a second thing that can be wrong.
type sellerContactBody struct {
	Email *string `json:"email" doc:"The seller's email, if they opted into sharing one."`
	Phone *string `json:"phone" doc:"The seller's phone number, if they opted into sharing one."`
}

func toSellerContactBody(contact model.SellerContact) sellerContactBody {
	return sellerContactBody{
		Email: contact.Email,
		Phone: contact.Phone,
	}
}

// registerSellerEndpoints mounts the contact reveal. It needs an actor even
// though the fields are ones the seller published: a signed-in caller is the one
// thing that separates "a buyer is asking about this card" from a script reading
// every seller in the database.
func registerSellerEndpoints(api huma.API, svc SellerService, actor ActorFunc) {
	huma.Register(api,
		newOp(http.MethodGet, "/sellers/{sellerId}/contact", "get-seller-contact",
			"How to reach a seller: only what they opted into sharing, which may be nothing.",
			http.StatusOK),
		func(ctx context.Context, req *sellerContactInput) (*sellerContactOutput, error) {
			if _, err := actor(ctx); err != nil {
				return nil, errNoActor
			}

			contact, err := svc.SellerContact(ctx, req.SellerID)
			if err != nil {
				return nil, handleErr(ctx, fmt.Errorf("getting a seller's contact details: %w", err))
			}
			return &sellerContactOutput{Body: toSellerContactBody(contact)}, nil
		})
}

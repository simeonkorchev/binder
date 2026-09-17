package model_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"github.com/simeonkorchev/binder/internal/user/model"
)

var _ = Describe("Contact.Normalise", func() {
	// Every field of Contact is exercised in both directions, so a third contact
	// detail added to the struct and forgotten in trimShared fails here rather
	// than silently going untrimmed (000-principles.md section 9).
	DescribeTable("what comes out",
		func(given model.Contact, wantEmail, wantPhone *string, wantBlank []model.ContactField) {
			normalised, blank := given.Normalise()

			Expect(normalised.Email).To(matchOptional(wantEmail))
			Expect(normalised.Phone).To(matchOptional(wantPhone))
			if len(wantBlank) == 0 {
				Expect(blank).To(BeEmpty())
			} else {
				Expect(blank).To(Equal(wantBlank))
			}
		},
		Entry("nothing shared stays nothing shared, and is not blank",
			model.Contact{Email: nil, Phone: nil}, nil, nil, nil),
		Entry("both shared and already clean come through unchanged",
			model.Contact{Email: ptr("seller@example.com"), Phone: ptr("+359888123456")},
			ptr("seller@example.com"), ptr("+359888123456"), nil),
		Entry("surrounding whitespace is trimmed off both",
			model.Contact{Email: ptr("  seller@example.com\n"), Phone: ptr("\t+359888123456 ")},
			ptr("seller@example.com"), ptr("+359888123456"), nil),
		Entry("an empty email is blank",
			model.Contact{Email: ptr(""), Phone: nil}, nil, nil,
			[]model.ContactField{model.ContactFieldEmail}),
		Entry("a space-only email is blank",
			model.Contact{Email: ptr("   "), Phone: nil}, nil, nil,
			[]model.ContactField{model.ContactFieldEmail}),
		Entry("a tab-only email is blank, which the column's own CHECK does not catch",
			model.Contact{Email: ptr("\t"), Phone: nil}, nil, nil,
			[]model.ContactField{model.ContactFieldEmail}),
		Entry("a newline-only phone number is blank",
			model.Contact{Email: nil, Phone: ptr("\n")}, nil, nil,
			[]model.ContactField{model.ContactFieldPhone}),
		Entry("both blank names both, in field order",
			model.Contact{Email: ptr(" "), Phone: ptr(" ")}, nil, nil,
			[]model.ContactField{model.ContactFieldEmail, model.ContactFieldPhone}),
		Entry("a good email beside a blank phone number keeps the email and names the phone",
			model.Contact{Email: ptr("seller@example.com"), Phone: ptr(" ")},
			ptr("seller@example.com"), nil,
			[]model.ContactField{model.ContactFieldPhone}),
	)

	It("does not write through to the value it was given", func() {
		original := "  seller@example.com  "
		given := model.Contact{Email: &original, Phone: nil}

		normalised, blank := given.Normalise()

		Expect(blank).To(BeEmpty())
		Expect(normalised.Email).To(HaveValue(Equal("seller@example.com")))
		Expect(original).To(Equal("  seller@example.com  "),
			"the caller's string must be left alone: a request body is not ours to edit")
	})
})

var _ = Describe("ValidAuthProvider", func() {
	DescribeTable("which providers this backend knows",
		func(provider model.AuthProvider, want bool) {
			Expect(model.ValidAuthProvider(provider)).To(Equal(want))
		},
		Entry("Apple", model.ProviderApple, true),
		Entry("Google", model.ProviderGoogle, true),
		Entry("a provider nobody wired up", model.AuthProvider("facebook"), false),
		Entry("the zero value, which is what an absent field arrives as", model.AuthProvider(""), false),
		Entry("the right name in the wrong case", model.AuthProvider("Apple"), false),
	)

	It("lists every provider the enum knows, so nothing derives the list twice", func() {
		Expect(model.AuthProviders()).To(Equal([]model.AuthProvider{
			model.ProviderApple, model.ProviderGoogle,
		}))
		for _, provider := range model.AuthProviders() {
			Expect(model.ValidAuthProvider(provider)).To(BeTrue())
		}
	})
})

// matchOptional matches a *string against an expected one, treating nil as its
// own outcome rather than as "any value".
func matchOptional(want *string) types.GomegaMatcher {
	if want == nil {
		return BeNil()
	}
	return HaveValue(Equal(*want))
}

// ptr is a pointer to a copy of v, for the optional contact fields.
func ptr[T any](v T) *T {
	return &v
}

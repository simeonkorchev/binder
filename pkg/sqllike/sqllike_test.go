package sqllike_test

import (
	"testing"

	"github.com/simeonkorchev/binder/pkg/sqllike"
)

// The table is the rule: the three characters LIKE reads as instructions come
// back escaped, everything else comes back untouched, and the backslash is
// escaped once rather than twice — which is what the replacement order buys.
//
// That a pattern built from this really matches the literal term is proven
// against Postgres in internal/card/store and internal/listing/store; this
// pins the string, which those suites cannot show apart from a query.
func TestEscapePattern(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		term string
		want string
	}{
		"a plain term is unchanged":        {term: "Dark Magician", want: "Dark Magician"},
		"an empty term is unchanged":       {term: "", want: ""},
		"a percent means itself":           {term: "100%", want: `100\%`},
		"an underscore means itself":       {term: "SDK_1", want: `SDK\_1`},
		"a backslash is escaped once":      {term: `a\b`, want: `a\\b`},
		"a backslash before a wildcard":    {term: `a\%b`, want: `a\\\%b`},
		"every metacharacter at once":      {term: `%_\`, want: `\%\_\\`},
		"a term that is only wildcards":    {term: "%%", want: `\%\%`},
		"punctuation LIKE does not read":   {term: "Ojama - Yellow!", want: "Ojama - Yellow!"},
		"a term with a quote is untouched": {term: `O'Brien`, want: `O'Brien`},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := sqllike.EscapePattern(tc.term); got != tc.want {
				t.Errorf("EscapePattern(%q) = %q, want %q", tc.term, got, tc.want)
			}
		})
	}
}

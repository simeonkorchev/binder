package store

import (
	"strings"
	"testing"
)

// White-box: placeholders is the one place in this repository that builds SQL
// with fmt, so what it may emit is pinned here rather than inferred from the
// statements that use it. 004-security.md permits it precisely because it
// interpolates loop indices and nothing else.
func TestPlaceholdersNumbersEveryColumnOfEveryRow(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		rows    int
		columns int
		want    string
	}{
		{name: "one row", rows: 1, columns: 2, want: "($1,$2)"},
		{name: "several rows", rows: 3, columns: 2, want: "($1,$2),($3,$4),($5,$6)"},
		{name: "four columns", rows: 2, columns: 4, want: "($1,$2,$3,$4),($5,$6,$7,$8)"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if got := placeholders(testCase.rows, testCase.columns); got != testCase.want {
				t.Errorf("placeholders(%d, %d) = %q, want %q",
					testCase.rows, testCase.columns, got, testCase.want)
			}
		})
	}
}

// The guard that matters: a placeholder list is digits and punctuation. If a
// value ever reached this function it would have to appear in the output, and
// this is what would go red.
func TestPlaceholdersEmitsOnlyPlaceholders(t *testing.T) {
	t.Parallel()

	got := placeholders(4, 3)
	if strings.Trim(got, "$0123456789(),") != "" {
		t.Errorf("placeholders emitted something that is not a placeholder: %q", got)
	}
}

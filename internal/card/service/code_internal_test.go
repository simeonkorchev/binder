package service

import "testing"

// parseScanCode is unexported and pure, so it is pinned here rather than through
// ResolveScan: these cases are about what OCR does to a printed code, and
// driving them through the ladder would say nothing extra about the split.
func TestParseScanCodeSplitsOCRTextTheWayTheDatabaseSplitsAStoredCode(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		text   string
		prefix string
		number string
	}{
		{name: "a clean printed code", text: "LOB-EN005", prefix: "LOB", number: "EN005"},
		{name: "lower case, as a camera often reads it", text: "lob-en005", prefix: "LOB", number: "EN005"},
		{name: "the hyphen dropped to a space", text: "LOB EN005", prefix: "LOB", number: "EN005"},
		{name: "an en dash instead of a hyphen", text: "LOB–EN005", prefix: "LOB", number: "EN005"},
		{name: "stray punctuation at both ends", text: " .LOB-EN005, ", prefix: "LOB", number: "EN005"},
		// The database's set_number is everything after the *first* hyphen, so
		// a second one stays in the number rather than being silently lost.
		{name: "a code with a second separator", text: "LOB-EN-005", prefix: "LOB", number: "EN-005"},
		// One surviving token is read as the number: that is the rung it can
		// still drive. A prefix read as a number simply matches nothing.
		{name: "only one half survived", text: "EN005", prefix: "", number: "EN005"},
		{name: "nothing alphanumeric at all", text: " -- ", prefix: "", number: ""},
		{name: "empty text", text: "", prefix: "", number: ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			got := parseScanCode(c.text)
			if got.prefix != c.prefix {
				t.Errorf("prefix of %q = %q, want %q", c.text, got.prefix, c.prefix)
			}
			if got.number != c.number {
				t.Errorf("number of %q = %q, want %q", c.text, got.number, c.number)
			}
		})
	}
}

// setCode reassembles what the exact rung matches whole, so it has to be the
// inverse of the split for any code the database would accept — the
// card_printings_set_code_shape CHECK is that same shape.
func TestSetCodeReassemblesAParsedCode(t *testing.T) {
	t.Parallel()

	for _, text := range []string{"LOB-EN005", "lob en005", "LOB-EN-005"} {
		if got := parseScanCode(text).setCode(); got != parseScanCode(got).setCode() {
			t.Errorf("setCode of %q = %q, which does not survive a second round trip", text, got)
		}
	}

	if got := parseScanCode("lob en005").setCode(); got != "LOB-EN005" {
		t.Errorf("setCode of %q = %q, want %q", "lob en005", got, "LOB-EN005")
	}
}

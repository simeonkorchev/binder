package service

import "strings"

// scanCode is an OCR-read printed code split the way the database splits a
// stored one: everything before the first separator is the prefix, everything
// after it is the number. 001_cards.sql generates both columns that way, so a
// code carrying a second hyphen keeps it in the number.
type scanCode struct {
	prefix string
	number string
}

// setCode reassembles the printed code the exact rung matches whole. Only
// meaningful when the prefix survived OCR.
func (c scanCode) setCode() string {
	return c.prefix + "-" + c.number
}

// parseScanCode normalises OCR text into a scanCode. Codes are imported
// upper-case and are never case-folded in SQL, so the folding happens here.
//
// Every run of characters that is neither a letter nor a digit is a separator:
// that covers the hyphen itself, an en dash, a space where the hyphen was
// dropped, and stray punctuation at either end. What it deliberately does not
// do is guess at character confusions (0/O, 1/I) — the rungs below the exact
// one are what save those, and substituting here would multiply every lookup.
func parseScanCode(text string) scanCode {
	tokens := strings.FieldsFunc(strings.ToUpper(text), isSeparator)

	switch len(tokens) {
	case 0:
		return scanCode{prefix: "", number: ""}
	case 1:
		// One token survived, so the separator and whichever half stood before
		// it are gone. Read it as the number: that is the rung the number alone
		// drives, and a prefix read as a number simply matches nothing.
		return scanCode{prefix: "", number: tokens[0]}
	default:
		return scanCode{prefix: tokens[0], number: strings.Join(tokens[1:], "-")}
	}
}

func isSeparator(r rune) bool {
	isLetter := r >= 'A' && r <= 'Z'
	isDigit := r >= '0' && r <= '9'
	return !isLetter && !isDigit
}

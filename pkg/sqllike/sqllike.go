// Package sqllike escapes a user-typed search term so that it means itself
// inside a SQL LIKE or ILIKE pattern.
//
// This is not about injection — a term always travels as a bound parameter, and
// 004-security.md forbids interpolating one. It is about meaning: a user
// searching for "100%" wants the three characters, not "anything starting with
// 100", and one typing "SDK_1" wants that underscore rather than any character
// in its place.
//
// It lives here, and not in a domain, because two stores escape terms for the
// same reason — internal/card/store searches card names, internal/listing/store
// searches the names of the cards for sale — and the rule about what a
// metacharacter means is one rule, not one per domain
// (000-principles.md section 6).
package sqllike

import "strings"

// EscapePattern neutralises the LIKE metacharacters in a user-supplied search
// term: the wildcards `%` and `_`, and the backslash that escapes them.
//
// The backslash is replaced first, because it is LIKE's own escape character —
// escaping it after the wildcards would escape the backslashes this function
// just added. Postgres takes `\` as the escape character for a pattern with no
// ESCAPE clause, which is how every caller writes theirs.
//
// The result is a *pattern fragment*: the caller still surrounds it with the
// wildcards it wants, e.g. `name ILIKE '%' || $1 || '%'`.
func EscapePattern(term string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(term)
}

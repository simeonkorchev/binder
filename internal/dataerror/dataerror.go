// Package dataerror holds the typed persistence errors a service branches on.
//
// A store never lets a driver error escape: sql.ErrNoRows, a unique violation
// and a foreign-key violation are conditions the caller has to tell apart, and
// telling them apart by matching on a driver's error string is how a schema
// rename becomes a silent behaviour change. Each condition is a type here, with
// a constructor the store calls and a predicate the service matches on
// (.claude/rules/002-go-conventions.md section 4.3).
//
// A service must use the predicates — IsMissingEntityError and friends — rather
// than errors.As on the concrete types, so the matching rule lives in one place.
package dataerror

package dataerror

import (
	"errors"
	"fmt"
)

// InvalidReferenceError reports that a write named a row in another table that
// does not exist, or that does not satisfy the relationship the schema
// requires. The reference is the thing the caller pointed at ("card"), not the
// constraint that caught it: the constraint name is the store's business, and
// a service that branched on it would break the day the schema is refactored.
type InvalidReferenceError struct {
	reference string
	cause     error
}

// WrapInvalidReferenceError wraps a foreign-key violation as the named
// reference being unusable.
func WrapInvalidReferenceError(reference string, cause error) *InvalidReferenceError {
	return &InvalidReferenceError{reference: reference, cause: cause}
}

// Reference names what the caller pointed at that could not be resolved.
func (e *InvalidReferenceError) Reference() string {
	return e.reference
}

func (e *InvalidReferenceError) Error() string {
	return fmt.Sprintf("invalid %s reference: %v", e.reference, e.cause)
}

func (e *InvalidReferenceError) Unwrap() error {
	return e.cause
}

// IsInvalidReferenceError reports whether err is, or wraps, an
// InvalidReferenceError for the given reference.
func IsInvalidReferenceError(err error, reference string) bool {
	var target *InvalidReferenceError
	return errors.As(err, &target) && target.reference == reference
}

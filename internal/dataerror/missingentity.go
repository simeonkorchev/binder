package dataerror

import (
	"errors"
	"fmt"
)

// MissingEntityError reports that a lookup for one specific entity by id found
// nothing. It is deliberately not used for a query that returns a collection:
// a collection that matches nothing is an empty slice and a nil error
// (000-principles.md section 8b).
type MissingEntityError struct {
	entity string
	cause  error
}

// WrapMissingEntityError wraps the driver's not-found error as the named
// entity being absent.
func WrapMissingEntityError(entity string, cause error) *MissingEntityError {
	return &MissingEntityError{entity: entity, cause: cause}
}

// Entity is the name of the entity that was not found, for the caller that
// turns this into a message.
func (e *MissingEntityError) Entity() string {
	return e.entity
}

func (e *MissingEntityError) Error() string {
	return fmt.Sprintf("%s not found: %v", e.entity, e.cause)
}

func (e *MissingEntityError) Unwrap() error {
	return e.cause
}

// IsMissingEntityError reports whether err is, or wraps, a MissingEntityError.
func IsMissingEntityError(err error) bool {
	var target *MissingEntityError
	return errors.As(err, &target)
}

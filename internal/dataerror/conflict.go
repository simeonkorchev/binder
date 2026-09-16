package dataerror

import (
	"errors"
	"fmt"
)

// ConflictError reports that a write lost a race with a concurrent one: the row
// it tried to create already exists. It is not the error for a request that was
// wrong when it was made — that is caught before the write — but for two
// correct requests that collided, which is why the natural response is a 409
// and a retry rather than a message telling the user to change something.
type ConflictError struct {
	subject string
	cause   error
}

// WrapConflictError wraps a uniqueness violation as a conflict over the named
// subject.
func WrapConflictError(subject string, cause error) *ConflictError {
	return &ConflictError{subject: subject, cause: cause}
}

// Subject names what was already taken.
func (e *ConflictError) Subject() string {
	return e.subject
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("%s already taken: %v", e.subject, e.cause)
}

func (e *ConflictError) Unwrap() error {
	return e.cause
}

// IsConflictError reports whether err is, or wraps, a ConflictError.
func IsConflictError(err error) bool {
	var target *ConflictError
	return errors.As(err, &target)
}

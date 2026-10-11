// Package errs declares sentinels and error types of each shape.
package errs

import "errors"

// ErrClosed reports a closed resource.
var ErrClosed = errors.New("errs: closed")

// Timeout is no sentinel, because its name does not start with Err.
var Timeout = errors.New("errs: timeout")

// ValueError reports a bad field, with a value receiver and without a cause.
type ValueError struct {
	// Field is the bad field.
	Field string
}

// Error returns the message of the error.
func (e ValueError) Error() string {
	return "errs: bad " + e.Field
}

// WrapError has a cause and no method Unwrap.
type WrapError struct {
	// Err is the cause of the error.
	Err error
}

// Error returns the message of the error.
func (e *WrapError) Error() string {
	return "errs: wrap"
}

// ListError has a method Unwrap that returns a list.
type ListError struct {
	// Errs are the causes of the error.
	Errs []error
}

// Error returns the message of the error.
func (e *ListError) Error() string {
	return "errs: list"
}

// Unwrap returns the causes of the error.
func (e *ListError) Unwrap() []error {
	return e.Errs
}

// BoxError is a generic error type.
type BoxError[T any] struct {
	// Value is the value of the error.
	Value T
}

// Error returns the message of the error.
func (e *BoxError[T]) Error() string {
	return "errs: box"
}

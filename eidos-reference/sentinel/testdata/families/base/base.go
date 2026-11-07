// Package base declares the sentinel that the package foreign wraps.
package base

import "errors"

// ErrNotFound reports a missing record.
var ErrNotFound = errors.New("base: not found")

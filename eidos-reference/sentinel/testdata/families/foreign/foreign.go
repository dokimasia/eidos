// Package foreign wraps the sentinel ErrNotFound of the package base, which
// is no ancestor in foreign, also where foreign declares an ErrNotFound of
// its own.
package foreign

import (
	"errors"
	"fmt"

	"base"
)

// The sentinels of the package.
var (
	// ErrMissing wraps the sentinel of base.
	ErrMissing = fmt.Errorf("%w: missing", base.ErrNotFound)

	// ErrNotFound reports a missing policy.
	ErrNotFound = errors.New("foreign: not found")
)

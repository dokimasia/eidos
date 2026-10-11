package catalog

import "errors"

// ErrNotFound reports a product that the catalog does not contain.
var ErrNotFound = errors.New("catalog: product not found")

// ErrConflict reports a product that the catalog contains already.
var ErrConflict = errors.New("catalog: product exists")

// NotFoundError reports the product that a lookup of the catalog did not
// find.
type NotFoundError struct {
	// ID identifies the product.
	ID string
	// Err is the cause of the error.
	Err error
}

// Error returns the message of the error.
func (e *NotFoundError) Error() string {
	return "catalog: product " + e.ID + " not found"
}

// Unwrap returns the cause of the error.
func (e *NotFoundError) Unwrap() error {
	return e.Err
}

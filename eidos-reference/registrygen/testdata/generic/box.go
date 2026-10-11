// Package generic declares a generic struct under the directive register.
package generic

// Box contains one value.
//
//+reference:register
type Box[T any] struct {
	// V is the value of the box.
	V T
}

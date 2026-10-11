// Package generic declares generic interfaces under the directive stub.
package generic

import "fmt"

// Box contains one value.
type Box[V any] struct {
	// Value is the value of the box.
	Value V
}

// TT is a type whose name starts with the name of a type parameter.
type TT struct{}

// Shelf keeps values of one type.
//
//+reference:stub
type Shelf[T any] interface {
	// Fill stores a list of values.
	Fill(values []T)
	// Wrap stores the value of a box.
	Wrap(b Box[T])
	// Tag stores a list of tags.
	Tag(tags []TT)
}

// Labels keeps values with a label, a bound that int does not satisfy.
//
//+reference:stub
type Labels[T fmt.Stringer] interface {
	// Add stores v.
	Add(v T)
}

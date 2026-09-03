// Package rules is the mini projection vocabulary.
package rules

import "example.test/dep"

// Shape is the mini shape.
type Shape struct{ T dep.T }

// Bound is the mini binding.
type Bound struct{}

// TypeOf folds nothing.
func (Bound) TypeOf(d dep.T) Shape { return Shape{T: d} }

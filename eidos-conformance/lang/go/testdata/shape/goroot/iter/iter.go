// Package iter declares the two types of the standard library's iter
// package that the shape fixture's source names.
package iter

// Seq is an iterator over values.
type Seq[V any] func(yield func(V) bool)

// Seq2 is an iterator over pairs of values.
type Seq2[K, V any] func(yield func(K, V) bool)

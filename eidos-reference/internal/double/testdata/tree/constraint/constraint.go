// Package constraint declares a constraint interface, which no double
// implements.
package constraint

// Number is the set of the numeric types that a sum accepts.
type Number interface {
	~int | ~float64
}

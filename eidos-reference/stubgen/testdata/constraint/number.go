// Package constraint declares a constraint interface under the directive
// stub, which no stub implements.
package constraint

// Number is the set of the numeric types that a sum accepts.
//
//+reference:stub
type Number interface {
	~int | ~float64
}

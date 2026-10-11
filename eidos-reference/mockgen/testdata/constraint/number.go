// Package constraint declares a constraint interface under the directive
// mock, which no mock implements.
package constraint

// Number is the set of the numeric types that a sum accepts.
//
//+reference:mock
type Number interface {
	~int | ~float64
}

// Package unresolved declares a struct under the directive builder whose
// param defaults names a function that the package does not declare.
package unresolved

// Address is the postal address of a customer.
//
//+reference:builder defaults=missingAddress
type Address struct {
	// Street is the street and the house number.
	Street string
}

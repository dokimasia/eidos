// Package generic declares generic structs under the directive builder.
package generic

import "fmt"

// Page is one page of a listing.
//
//+reference:builder
type Page[T any] struct {
	// Items are the items of the page.
	Items []T
	// Next is the cursor of the next page, and empty on the last page.
	Next string `default:"end"`
}

// Labelled is a value with a label that tells it apart.
//
//+reference:builder
type Labelled[T fmt.Stringer] struct {
	// Value is the labelled value.
	Value T
}

// Package bad declares a struct under the directive builder whose default
// is no value of the type of its field.
package bad

// Shelf is a shelf of the store.
//
//+reference:builder
type Shelf struct {
	// Slots is the number of slots of the shelf.
	Slots int `default:"many"`
}

package catalog

import measure "example.com/shop/units"

// Ruler measures the items of the catalog.
//
//+reference:mock
type Ruler interface {
	// Length returns the length of the item at the position.
	Length(item string) measure.Meters
}

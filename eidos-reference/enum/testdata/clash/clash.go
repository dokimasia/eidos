// Package clash declares an enumeration whose variants share a text.
package clash

// Clash is an enumeration with two variants of one text.
//
//+reference:enum
type Clash string

// The variants of Clash.
const (
	// ClashFirst is the first variant.
	ClashFirst Clash = "same"
	// ClashSecond is the second variant, with the text of the first.
	ClashSecond Clash = "same"
)

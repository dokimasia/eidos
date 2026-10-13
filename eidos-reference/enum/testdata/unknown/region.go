// Package unknown declares an enumeration of text with a value that another
// package decides.
package unknown

import "runtime"

// Region is an enumeration of text whose second value is the operating
// system of the build.
//
//+reference:enum
type Region string

// The variants of Region.
const (
	// RegionEU is the region eu.
	RegionEU Region = "eu"
	// RegionHost is the region of the operating system of the build.
	RegionHost Region = Region(runtime.GOOS)
)

package catalog

import "time"

// Clock reads the time of the catalog in one unit.
//
//+reference:mock
type Clock[D ~int32 | time.Duration] interface {
	// Now returns the time.
	Now() D
}

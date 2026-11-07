// Package cycle declares two sentinels that wrap each other, which Go
// refuses to initialize, and a sentinel that wraps one of them. Its tree has
// no module file, so no build of a module compiles it.
package cycle

import "fmt"

// The sentinels that wrap each other, and a sentinel that wraps one of them.
var (
	// ErrFirst wraps ErrSecond.
	ErrFirst = fmt.Errorf("%w: first", ErrSecond)

	// ErrSecond wraps ErrFirst.
	ErrSecond = fmt.Errorf("%w: second", ErrFirst)

	// ErrStart wraps ErrFirst.
	ErrStart = fmt.Errorf("%w: start", ErrFirst)
)

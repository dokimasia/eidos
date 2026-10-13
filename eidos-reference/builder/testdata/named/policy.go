// Package named declares a struct under the directive builder whose
// defaults are the names of declarations and a call of a function.
package named

import "time"

// Retries is the number of attempts of a policy without its own.
const Retries = 3

// Clock returns the current time.
type Clock interface {
	// Now returns the current time.
	Now() time.Time
}

// SystemClock returns the clock of the system.
func SystemClock() Clock { return nil }

// Policy is a retry policy.
//
//+reference:builder
type Policy struct {
	// Timeout is the time that one attempt waits.
	Timeout time.Duration `default:"time.Second"`
	// Attempts is the number of attempts.
	Attempts int `default:"Retries"`
	// Clock is the clock that measures the timeout.
	Clock Clock `default:"SystemClock()"`
}

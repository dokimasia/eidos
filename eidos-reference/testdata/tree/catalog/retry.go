package catalog

import "time"

// DefaultAttempts is the number of attempts of a retry policy without its
// own.
const DefaultAttempts = 3

// RetryPolicy is the policy of the retries of a request to the catalog.
//
//+reference:builder
type RetryPolicy struct {
	// Timeout is the time that one attempt waits.
	Timeout time.Duration `default:"time.Second"`
	// Attempts is the number of attempts.
	Attempts int `default:"DefaultAttempts"`
}

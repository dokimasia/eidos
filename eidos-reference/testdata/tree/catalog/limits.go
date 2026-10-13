package catalog

import "time"

// Limits bounds the requests of a client of the catalog.
//
//+reference:builder
type Limits struct {
	// Retry bounds the retries of one request.
	Retry struct {
		Wait time.Duration
		Max  int
	}
	// Burst is the number of requests without a wait.
	Burst int
}

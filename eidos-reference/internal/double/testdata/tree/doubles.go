// Package doubles declares the interfaces that the tests of the doubles
// mirror.
package doubles

import "io"

// Ledger has a method with unnamed parameters, and a method with a
// parameter that has the name of the receiver of its double.
type Ledger interface {
	// Get returns the entry of a key.
	Get(string) (int, error)
	// Put stores the entry l under the key k.
	Put(l int, k string) error
}

// Counter has a method with a result that has the name of the receiver of
// its double.
type Counter interface {
	// Count returns the number c of the entries.
	Count() (c int)
}

// Store has a method whose result has the name that the position of an
// unnamed parameter gives.
type Store interface {
	// Put stores a value under a key and returns the previous value a1.
	Put(string, int) (a1 int)
}

// Stream embeds an interface that the load does not resolve.
type Stream interface {
	io.Reader
	// Close ends the stream.
	Close() error
}

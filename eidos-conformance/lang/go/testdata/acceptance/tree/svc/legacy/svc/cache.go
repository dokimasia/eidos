// Package svc keeps the sessions of the previous console. Its name is the
// name of the package svc of the current console, so a file that imports
// both packages qualifies one of them by an alias.
package svc

// Entry is one cached session.
type Entry struct {
	Key string
}

// Cache is the cache seam of the previous console.
//
//+acme:stub
type Cache interface {
	Lookup(key string) (Entry, error)
}

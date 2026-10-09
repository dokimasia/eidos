// Package context declares the one type of the standard library's
// context package that the shape fixture's source names.
package context

// Context is the deadline and the cancellation of a call.
type Context interface {
	Done() <-chan struct{}
}

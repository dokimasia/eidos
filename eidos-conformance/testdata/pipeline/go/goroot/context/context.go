// Package context declares the one type of the standard library's
// context package the pipeline fixture's source names.
package context

// Context carries a deadline, a cancellation signal and request-scoped
// values across API boundaries.
type Context interface {
	Done() <-chan struct{}
}

// Package shapes declares structs with a method Write of each shape.
package shapes

// Plain declares the method Write of an io.Writer.
type Plain struct{}

// Write discards p.
func (Plain) Write(p []byte) (int, error) { return len(p), nil }

// Tee embeds Plain, which promotes its method Write.
type Tee struct {
	Plain
}

// Text writes a string.
type Text struct{}

// Write discards s.
func (Text) Write(s string) (int, error) { return len(s), nil }

// Spread takes the bytes one by one.
type Spread struct{}

// Write discards p.
func (Spread) Write(p ...byte) (int, error) { return len(p), nil }

// Single returns the count alone.
type Single struct{}

// Write discards p.
func (Single) Write(p []byte) int { return len(p) }

// Wide returns a count of 64 bits.
type Wide struct{}

// Write discards p.
func (Wide) Write(p []byte) (int64, error) { return int64(len(p)), nil }

// Unsigned returns an unsigned count.
type Unsigned struct{}

// Write discards p.
func (Unsigned) Write(p []byte) (uint, error) { return uint(len(p)), nil }

// Flagged returns a flag in place of the error.
type Flagged struct{}

// Write discards p.
func (Flagged) Write(p []byte) (int, bool) { return len(p), true }

// Closer declares a method other than Write.
type Closer struct{}

// Close does nothing.
func (Closer) Close() error { return nil }

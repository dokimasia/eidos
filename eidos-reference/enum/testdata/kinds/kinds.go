// Package kinds declares enumerations of each kind under the directive
// enum.
package kinds

// Level is a numeric enumeration.
//
//+reference:enum
type Level int

// The levels.
const (
	// LevelLow is the low level.
	LevelLow Level = iota
	// LevelHigh is the high level.
	LevelHigh
)

// Mode is an enumeration that declares its own method String.
//
//+reference:enum
type Mode int

// The modes.
const (
	// ModeRead reads.
	ModeRead Mode = iota
	// ModeWrite writes.
	ModeWrite
)

// String returns the name of m.
func (m Mode) String() string {
	if m == ModeRead {
		return "read"
	}
	return "write"
}

// Quiet is an enumeration whose directive withholds the methods.
//
//+reference:enum methods=false
type Quiet int

// The settings of Quiet.
const (
	// QuietOn is on.
	QuietOn Quiet = iota
	// QuietOff is off.
	QuietOff
)

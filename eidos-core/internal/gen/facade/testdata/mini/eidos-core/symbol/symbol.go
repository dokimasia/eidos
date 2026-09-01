// Package symbol is the mini vocabulary.
package symbol

// Kind is a declaration's kind.
type Kind uint8

// The kinds the mini kernel knows.
const (
	// KindStruct is a struct.
	KindStruct Kind = iota
	// KindEnum is an enum.
	KindEnum
)

// Generic carries one value of any type.
type Generic[T any] struct{ v T }

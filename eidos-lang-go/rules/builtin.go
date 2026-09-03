// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The builtin and standard spellings the rules classify.
const (
	spellInt      = "int"
	spellInt8     = "int8"
	spellInt16    = "int16"
	spellInt32    = "int32"
	spellInt64    = "int64"
	spellRune     = "rune"
	spellUint     = "uint"
	spellUintptr  = "uintptr"
	spellUint8    = "uint8"
	spellByte     = "byte"
	spellUint16   = "uint16"
	spellUint32   = "uint32"
	spellUint64   = "uint64"
	spellFloat32  = "float32"
	spellFloat64  = "float64"
	spellString   = "string"
	spellAny      = "any"
	spellTime     = "time.Time"
	spellDuration = "time.Duration"
)

// Builtin classifies a named reference the resolution step left
// without a target, by its spelling: the integer and float
// spellings as scalars with their widths, byte and rune through
// the widths they alias, bool and string as their leaves,
// time.Time and time.Duration through the well-known registry, and
// every other spelling, any, error and comparable included, as
// Opaque.
func (Rules) Builtin(ref *node.TypeRef, _ rules.View) rules.TypeShape {
	if ref == nil {
		return rules.Opaque(nil)
	}
	spelling := named(ref)
	switch spelling {
	case spellInt:
		return rules.Scalar(ref.Spelling, rules.ScalarInt, 0)
	case spellInt8:
		return rules.Scalar(ref.Spelling, rules.ScalarInt, 8)
	case spellInt16:
		return rules.Scalar(ref.Spelling, rules.ScalarInt, 16)
	case spellInt32, spellRune:
		return rules.Scalar(ref.Spelling, rules.ScalarInt, 32)
	case spellInt64:
		return rules.Scalar(ref.Spelling, rules.ScalarInt, 64)
	case spellUint, spellUintptr:
		return rules.Scalar(ref.Spelling, rules.ScalarUint, 0)
	case spellUint8, spellByte:
		return rules.Scalar(ref.Spelling, rules.ScalarUint, 8)
	case spellUint16:
		return rules.Scalar(ref.Spelling, rules.ScalarUint, 16)
	case spellUint32:
		return rules.Scalar(ref.Spelling, rules.ScalarUint, 32)
	case spellUint64:
		return rules.Scalar(ref.Spelling, rules.ScalarUint, 64)
	case spellFloat32:
		return rules.Scalar(ref.Spelling, rules.ScalarFloat, 32)
	case spellFloat64:
		return rules.Scalar(ref.Spelling, rules.ScalarFloat, 64)
	case boolSpelling:
		return rules.Leaf(symbol.FormBool, ref.Spelling)
	case spellString:
		return rules.Leaf(symbol.FormText, ref.Spelling)
	case spellTime:
		return rules.Reference(ref.Spelling, rules.WellKnownTimestamp)
	case spellDuration:
		return rules.Reference(ref.Spelling, rules.WellKnownDuration)
	default:
		return rules.Opaque(ref)
	}
}

// numeric says which builtin spellings are numbers, for the value
// table and the comparability rule.
func numeric(spelling string) bool {
	switch spelling {
	case spellInt, spellInt8, spellInt16, spellInt32, spellInt64, spellRune,
		spellUint, spellUintptr, spellUint8, spellByte, spellUint16, spellUint32, spellUint64,
		spellFloat32, spellFloat64:
		return true
	default:
		return false
	}
}

// floating says which numeric spellings are floats.
func floating(spelling string) bool {
	return spelling == spellFloat32 || spelling == spellFloat64
}

// comparableBuiltin says which builtin spellings Go compares with
// ==: the numbers, string, bool, the complex numbers, and the
// interfaces any, error and comparable, which compare at run time.
func comparableBuiltin(spelling string) bool {
	if numeric(spelling) {
		return true
	}
	switch spelling {
	case spellString,
		boolSpelling,
		"complex64",
		"complex128",
		spellAny,
		errorSpelling,
		"comparable":
		return true
	default:
		return false
	}
}

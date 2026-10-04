// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The predeclared spellings the rules classify.
const (
	spellInt     = "int"
	spellInt8    = "int8"
	spellInt16   = "int16"
	spellInt32   = "int32"
	spellInt64   = "int64"
	spellRune    = "rune"
	spellUint    = "uint"
	spellUintptr = "uintptr"
	spellUint8   = "uint8"
	spellByte    = "byte"
	spellUint16  = "uint16"
	spellUint32  = "uint32"
	spellUint64  = "uint64"
	spellFloat32 = "float32"
	spellFloat64 = "float64"
	spellString  = "string"
	spellAny     = "any"
)

// The standard library types the rules classify, each by the path of
// the package that declares it and its name, so a reference through
// any alias of the import classifies.
const (
	timePackage   = "time"
	timeName      = "Time"
	durationName  = "Duration"
	unsafePackage = "unsafe"
	pointerName   = "Pointer"
)

// typeName is a type a reference names without a target: the path
// of the package its import names, empty for a predeclared type and
// for a reference no import qualifies, and the name.
type typeName struct{ pkg, name string }

// The types the rules match a reference against.
var (
	timeType          = typeName{pkg: timePackage, name: timeName}
	durationType      = typeName{pkg: timePackage, name: durationName}
	unsafePointerType = typeName{pkg: unsafePackage, name: pointerName}
	anyType           = typeName{name: spellAny}
	errorType         = typeName{name: errorSpelling}
)

// nameOf returns the type a reference names: the import path the
// frontend recorded with the name behind the qualifier, and an empty
// path with the whole spelling for a reference no import qualifies.
func nameOf(ref *node.TypeRef) typeName {
	spelling := named(ref)
	if ref == nil || ref.Package == "" {
		return typeName{name: spelling}
	}
	return typeName{pkg: ref.Package, name: golang.Unqualified(spelling)}
}

// Builtin classifies a named reference the resolution step left
// without a target: the integer and float spellings as scalars with
// their widths, byte and rune through the widths they alias, bool and
// string as their leaves, time.Time and time.Duration through the
// well-known registry, and every other reference, any, error and
// comparable included, as Opaque. A predeclared spelling classifies
// only where no import qualifies it, and the two time types by the
// package their import names and their name, under any alias. It
// allocates nothing.
func (Rules) Builtin(ref *node.TypeRef, _ rules.View) rules.TypeShape {
	if ref == nil {
		return rules.Opaque(nil)
	}
	switch nameOf(ref) {
	case typeName{name: spellInt}:
		return rules.Scalar(ref.Spelling, rules.ScalarInt, 0)
	case typeName{name: spellInt8}:
		return rules.Scalar(ref.Spelling, rules.ScalarInt, 8)
	case typeName{name: spellInt16}:
		return rules.Scalar(ref.Spelling, rules.ScalarInt, 16)
	case typeName{name: spellInt32}, typeName{name: spellRune}:
		return rules.Scalar(ref.Spelling, rules.ScalarInt, 32)
	case typeName{name: spellInt64}:
		return rules.Scalar(ref.Spelling, rules.ScalarInt, 64)
	case typeName{name: spellUint}, typeName{name: spellUintptr}:
		return rules.Scalar(ref.Spelling, rules.ScalarUint, 0)
	case typeName{name: spellUint8}, typeName{name: spellByte}:
		return rules.Scalar(ref.Spelling, rules.ScalarUint, 8)
	case typeName{name: spellUint16}:
		return rules.Scalar(ref.Spelling, rules.ScalarUint, 16)
	case typeName{name: spellUint32}:
		return rules.Scalar(ref.Spelling, rules.ScalarUint, 32)
	case typeName{name: spellUint64}:
		return rules.Scalar(ref.Spelling, rules.ScalarUint, 64)
	case typeName{name: spellFloat32}:
		return rules.Scalar(ref.Spelling, rules.ScalarFloat, 32)
	case typeName{name: spellFloat64}:
		return rules.Scalar(ref.Spelling, rules.ScalarFloat, 64)
	case boolType:
		return rules.Leaf(symbol.FormBool, ref.Spelling)
	case typeName{name: spellString}:
		return rules.Leaf(symbol.FormText, ref.Spelling)
	case timeType:
		return rules.Reference(ref.Spelling, rules.WellKnownTimestamp)
	case durationType:
		return rules.Reference(ref.Spelling, rules.WellKnownDuration)
	default:
		return rules.Opaque(ref)
	}
}

// comparableBuiltin reports whether Go compares a reference the
// resolution step left without a target with ==: every predeclared
// type does, the interfaces any, error and comparable at run time,
// and so does unsafe.Pointer.
func comparableBuiltin(ref *node.TypeRef) bool {
	t := nameOf(ref)
	return t.pkg == "" && golang.Predeclared(t.name) || t == unsafePointerType
}

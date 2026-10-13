// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"strings"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The predeclared spellings the rules classify by name. The numeric
// types classify through [golang.NumericOf], except the complex numbers,
// which no other language has.
const (
	spellInt        = "int"
	spellRune       = "rune"
	spellUint8      = "uint8"
	spellByte       = "byte"
	spellString     = "string"
	spellAny        = "any"
	spellComplex64  = "complex64"
	spellComplex128 = "complex128"
)

// The tokens of the empty inline interface, which white space can
// separate.
const (
	bodyOpen  = "{"
	bodyClose = "}"
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

// The packages of the standard library whose types [stdComparable] lists
// more than twice.
const (
	netipPackage = "net/netip"
	syncPackage  = "sync"
	bigPackage   = "math/big"
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
	stringType        = typeName{name: spellString}
	complex64Type     = typeName{name: spellComplex64}
	complex128Type    = typeName{name: spellComplex128}
)

// stdComparable is the table of the struct types of the standard library
// that a struct field commonly has, with whether Go compares each with
// ==. The rules read it for a reference that the load does not place, and
// for a declaration whose lowering left out a field, such as a struct of
// the standard library at the signature depth.
var stdComparable = map[typeName]bool{
	timeType:                              true,
	{pkg: timePackage, name: "Location"}:  false,
	{pkg: netipPackage, name: "Addr"}:     true,
	{pkg: netipPackage, name: "AddrPort"}: true,
	{pkg: netipPackage, name: "Prefix"}:   true,
	{pkg: "net/url", name: "URL"}:         true,
	{pkg: "net/url", name: "Userinfo"}:    true,
	{pkg: syncPackage, name: "Mutex"}:     true,
	{pkg: syncPackage, name: "RWMutex"}:   true,
	{pkg: syncPackage, name: "Once"}:      true,
	{pkg: syncPackage, name: "WaitGroup"}: true,
	{pkg: bigPackage, name: "Int"}:        false,
	{pkg: bigPackage, name: "Float"}:      false,
	{pkg: bigPackage, name: "Rat"}:        false,
	{pkg: "bytes", name: "Buffer"}:        false,
	{pkg: "strings", name: "Builder"}:     false,
	{pkg: "regexp", name: "Regexp"}:       false,
}

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
// string as their leaves, any as the top type, error, time.Time and
// time.Duration through the well-known registry, and every other
// reference, comparable and the complex numbers included, as Opaque. A
// predeclared spelling classifies only where no import qualifies it, and
// the two time types by the package their import names and their name,
// under any alias. An inline reference classifies as the top type where
// it is the empty interface, interface{}, and as Opaque otherwise, which
// the kernel's fold keeps inline. It allocates nothing.
func (Rules) Builtin(ref *node.TypeRef, _ rules.View) rules.TypeShape {
	switch {
	case ref == nil:
		return rules.Opaque(nil)
	case ref.Form == symbol.FormInline && emptyInterfaceBody(ref.Spelling):
		return rules.Leaf(symbol.FormDynamic, ref.Spelling)
	case ref.Form == symbol.FormInline:
		return rules.Opaque(ref)
	}
	name := nameOf(ref)
	if n, numeric := golang.NumericOf(name.name); numeric && name.pkg == "" {
		return rules.Scalar(ref.Spelling, n.Class, n.Width)
	}
	switch name {
	case boolType:
		return rules.Leaf(symbol.FormBool, ref.Spelling)
	case stringType:
		return rules.Leaf(symbol.FormText, ref.Spelling)
	case anyType:
		return rules.Leaf(symbol.FormDynamic, ref.Spelling)
	case errorType:
		return rules.Reference(ref.Spelling, rules.WellKnownError)
	case timeType:
		return rules.Reference(ref.Spelling, rules.WellKnownTimestamp)
	case durationType:
		return rules.Reference(ref.Spelling, rules.WellKnownDuration)
	default:
		return rules.Opaque(ref)
	}
}

// WellKnown returns the well-known identity of the declarations Time and
// Duration of the package time, and reports false for every other
// declaration. A run that loads the standard library gives a reference
// to either a target, and the fold then classifies the target through
// WellKnown, as [Rules.Builtin] classifies a reference without one. It
// allocates nothing.
func (Rules) WellKnown(id symbol.Identity) (symbol.Identity, bool) {
	switch (typeName{pkg: id.Package, name: id.Name}) {
	case timeType:
		return rules.WellKnownTimestamp, true
	case durationType:
		return rules.WellKnownDuration, true
	default:
		return symbol.Identity{}, false
	}
}

// comparableBuiltin reports whether Go compares a reference the
// resolution step left without a target with ==: every predeclared
// type does, the interfaces any, error and comparable at run time,
// and so do unsafe.Pointer, time.Duration and each comparable type of
// [stdComparable].
func comparableBuiltin(ref *node.TypeRef) bool {
	t := nameOf(ref)
	return t.pkg == "" && golang.Predeclared(t.name) || t == unsafePointerType || t == durationType ||
		stdComparable[t]
}

// emptyInterfaceBody reports whether the spelling of an inline reference
// is the interface without elements, interface{}, with any white space
// between its tokens. It allocates nothing.
func emptyInterfaceBody(spelling string) bool {
	rest, keyword := strings.CutPrefix(strings.TrimSpace(spelling), interfaceKeyword)
	if !keyword {
		return false
	}
	body, opened := strings.CutPrefix(strings.TrimSpace(rest), bodyOpen)
	return opened && strings.TrimSpace(body) == bodyClose
}

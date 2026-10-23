// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package golang

import (
	"go/token"
	"go/types"
	"path"
	"strings"
	"unicode"
	"unicode/utf8"

	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/rules"
)

// The alias spellings of Go's two import forms that bind no
// qualifier.
const (
	// BlankAlias imports a package for its side effects alone.
	BlankAlias = "_"
	// DotAlias merges a package's exported names into the file's
	// scope.
	DotAlias = "."
)

// IterPackage is the path of the standard library's iterator package,
// and IterSeq and IterSeq2 name the two sequence types it declares,
// which a callable returns.
const (
	IterPackage = "iter"
	IterSeq     = "Seq"
	IterSeq2    = "Seq2"
)

// The spellings the assumed import name strips.
const (
	// majorPrefix opens a major-version path element, such as v5.
	majorPrefix = "v"
	// goPrefix opens a repository name that is not the package's,
	// such as go-sqlite3.
	goPrefix = "go-"
)

// The characters of the integer of a major-version path element.
const (
	// signMarks are the signs that can open the integer.
	signMarks = "+-"
	// decimalDigits are the digits of the integer.
	decimalDigits = "0123456789"
)

// The escapes that make an assumed import name a qualifier.
const (
	// qualifierPrefix opens the qualifier of a name that starts with a
	// digit, and is the qualifier of a path whose assumed name is empty.
	qualifierPrefix = "pkg"
	// keywordSuffix closes the qualifier of a name that is a keyword.
	keywordSuffix = "_"
)

// Numeric is the class and the width in bits of one of Go's predeclared
// numeric types. A width of 0 is the platform's width.
type Numeric struct {
	Class rules.ScalarClass
	Width int
}

// numericKinds lists the basic kinds of Go's predeclared numeric types
// with their class and width, in the order that [NumericName] searches.
// uintptr follows uint, so the platform's unsigned width is named uint.
var numericKinds = [...]struct {
	kind types.BasicKind
	Numeric
}{
	{types.Int, Numeric{Class: rules.ScalarInt, Width: 0}},
	{types.Int8, Numeric{Class: rules.ScalarInt, Width: 8}},
	{types.Int16, Numeric{Class: rules.ScalarInt, Width: 16}},
	{types.Int32, Numeric{Class: rules.ScalarInt, Width: 32}},
	{types.Int64, Numeric{Class: rules.ScalarInt, Width: 64}},
	{types.Uint, Numeric{Class: rules.ScalarUint, Width: 0}},
	{types.Uint8, Numeric{Class: rules.ScalarUint, Width: 8}},
	{types.Uint16, Numeric{Class: rules.ScalarUint, Width: 16}},
	{types.Uint32, Numeric{Class: rules.ScalarUint, Width: 32}},
	{types.Uint64, Numeric{Class: rules.ScalarUint, Width: 64}},
	{types.Uintptr, Numeric{Class: rules.ScalarUint, Width: 0}},
	{types.Float32, Numeric{Class: rules.ScalarFloat, Width: 32}},
	{types.Float64, Numeric{Class: rules.ScalarFloat, Width: 64}},
}

// NumericOf returns the class and the width of a predeclared numeric
// type, and false for any other name. byte and rune are the aliases of
// uint8 and int32, and uintptr has the platform's width. The universe
// scope of go/types decides which type a name denotes. NumericOf
// allocates nothing.
func NumericOf(name string) (Numeric, bool) {
	tn, is := types.Universe.Lookup(name).(*types.TypeName)
	if !is {
		return Numeric{}, false
	}
	basic, is := tn.Type().(*types.Basic)
	if !is {
		return Numeric{}, false
	}
	for _, k := range numericKinds {
		if k.kind == basic.Kind() {
			return k.Numeric, true
		}
	}
	return Numeric{}, false
}

// Predeclared reports whether a name is one of Go's predeclared
// types: a type name of the universe scope, which no package
// declares and no import qualifies, such as int, error, any and
// comparable. The universe scope of go/types is the authority, so
// the set follows the toolchain the module builds with. It allocates
// nothing.
func Predeclared(name string) bool {
	_, is := types.Universe.Lookup(name).(*types.TypeName)
	return is
}

// Basic reports whether a name is one of Go's predeclared basic
// types: a boolean, a number or a string. The predeclared
// interfaces any, error and comparable are not basic. It allocates
// nothing.
func Basic(name string) bool {
	_, is := basicInfo(name)
	return is
}

// Ordered reports whether a name is a predeclared basic type whose
// values order with <: an integer, a float or a string, the set
// cmp.Ordered admits. It allocates nothing.
func Ordered(name string) bool {
	info, is := basicInfo(name)
	return is && info&types.IsOrdered != 0
}

// basicInfo returns the properties of a predeclared basic type, and
// false for any other name.
func basicInfo(name string) (types.BasicInfo, bool) {
	tn, is := types.Universe.Lookup(name).(*types.TypeName)
	if !is {
		return 0, false
	}
	basic, is := tn.Type().Underlying().(*types.Basic)
	if !is {
		return 0, false
	}
	return basic.Info(), true
}

// NumericName returns the name of the predeclared numeric type of n,
// and false where Go declares none. An unsigned integer of 8 bits is
// uint8 and a signed one of 32 bits is int32, so the aliases byte and
// rune never return. NumericName allocates nothing.
func NumericName(n Numeric) (string, bool) {
	for _, k := range numericKinds {
		if k.Numeric == n {
			return types.Typ[k.kind].Name(), true
		}
	}
	return "", false
}

// AssumedName returns the name an import path binds when the import
// states no alias, by the rule goimports applies: the last path
// element, or the element before it where the last is a major
// version such as v5, without a go- prefix, cut at the first
// character that cannot continue an identifier. gopkg.in/yaml.v3
// binds yaml, github.com/golang-jwt/jwt/v5 binds jwt, and
// github.com/mattn/go-sqlite3 binds sqlite3.
//
// The rule reads the path alone. A package clause that declares
// another name binds that name instead, and the path does not show
// it. The name is part of the path, so AssumedName allocates nothing.
func AssumedName(importPath string) string {
	name := path.Base(importPath)
	if majorVersion(name) {
		if dir := path.Dir(importPath); dir != "." {
			name = path.Base(dir)
		}
	}
	name = strings.TrimPrefix(name, goPrefix)
	if cut := strings.IndexFunc(name, notIdentifier); cut >= 0 {
		name = name[:cut]
	}
	return name
}

// Qualifier returns the name under which a generated file imports the
// package of importPath: the name that [AssumedName] returns, escaped
// where it is no Go identifier. A keyword gains a trailing underscore, so
// example.com/shop/go binds go_. A name that starts with a digit gains the
// prefix pkg, so example.com/2fa binds pkg2fa. A path whose assumed name is
// empty binds pkg.
//
// An escaped qualifier differs from the last element of the path, so the
// import states it, and the qualifier binds whatever name the package
// declares. Qualifier allocates an escaped qualifier, one allocation, and
// nothing for a name that does not need an escape.
func Qualifier(importPath string) string {
	name := AssumedName(importPath)
	if name == "" {
		return qualifierPrefix
	}
	if token.IsKeyword(name) {
		return name + keywordSuffix
	}
	if first, _ := utf8.DecodeRuneInString(name); unicode.IsDigit(first) {
		return qualifierPrefix + name
	}
	return name
}

// Unqualified returns the name a type spelling qualifies: the text
// after the qualifier, and the whole spelling where none is written.
// A reference keeps an instantiation's arguments apart from its
// spelling, so a reference's spelling returns a bare name. It
// allocates nothing.
func Unqualified(spelling string) string {
	return spelling[strings.LastIndexByte(spelling, '.')+1:]
}

// ImportName returns the qualifier an import binds in its file: the
// alias the import states, or the name its path assumes. A blank
// import and a dot import bind no qualifier and report false, as
// does a nil import. It allocates nothing.
func ImportName(imp *node.Import) (string, bool) {
	switch {
	case imp == nil, imp.Wildcard, imp.Alias == BlankAlias, imp.Alias == DotAlias:
		return "", false
	case imp.Alias != "":
		return imp.Alias, true
	default:
		return AssumedName(imp.Path), true
	}
}

// majorVersion reports whether a path element is a major-version
// suffix: v followed by an integer, which is an optional sign and one or
// more decimal digits. It allocates nothing.
func majorVersion(elem string) bool {
	digits, versioned := strings.CutPrefix(elem, majorPrefix)
	if !versioned {
		return false
	}
	if digits != "" && strings.IndexByte(signMarks, digits[0]) >= 0 {
		digits = digits[1:]
	}
	return digits != "" && strings.TrimLeft(digits, decimalDigits) == ""
}

// notIdentifier reports whether a rune cannot continue a Go
// identifier: anything but a letter, a digit or an underscore.
func notIdentifier(r rune) bool {
	return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_'
}

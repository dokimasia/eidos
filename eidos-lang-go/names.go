// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package golang

import (
	"go/types"
	"path"
	"strconv"
	"strings"
	"unicode"

	"go.dokimi.dev/eidos/sdk/node"
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
// suffix: v followed by an integer.
func majorVersion(elem string) bool {
	digits, versioned := strings.CutPrefix(elem, majorPrefix)
	if !versioned {
		return false
	}
	_, err := strconv.Atoi(digits)
	return err == nil
}

// notIdentifier reports whether a rune cannot continue a Go
// identifier: anything but a letter, a digit or an underscore.
func notIdentifier(r rune) bool {
	return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_'
}

// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	"strings"

	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// valueSet is a defined type that can promote, and whether a test file
// declares it.
type valueSet struct {
	alias *node.Alias
	test  bool
}

// promoteEnums rewrites the idiomatic value sets of one package into the
// Enum kind. A value set is a defined type without type parameters, whose
// target is a name without type arguments, and a constant of the package
// that has the type as its declared type. Go gives a constant a boolean, a
// numeric or a string type, so the underlying type of such a type is one of
// these. The type becomes one Enum in its own file:
//
//   - the constants become its variants in the order of the files and of
//     their declarations, with the spellings of their values verbatim,
//     iota arithmetic included;
//   - the target of the defined type, such as uint8, becomes its
//     underlying type.
//
// A constant of a test file becomes a variant only of a type that a test
// file declares, because the go command builds every other file without
// the test files. A type name that two files declare promotes at its first
// declaration, the one that the load keeps. [foldMethods] folds the methods
// of an enumeration afterwards.
//
// The rewrite runs after every file of the package lowers. It returns two
// maps of what it replaced. The first maps each alias to its enum, and the
// deferred underlying stamps read it. The second maps each consumed
// declaration to its replacement, and the caller re-homes the recorded
// attachments and stamps through it.
func promoteEnums(
	files []*node.File,
) (map[*node.Alias]*node.Enum, map[symbol.Symbol]symbol.Symbol) {
	sets := map[string]valueSet{}
	for _, file := range files {
		test := strings.HasSuffix(file.Path, golang.TestSuffix+golang.Extension)
		for _, decl := range file.Decls {
			alias, is := decl.(*node.Alias)
			if !is || !alias.Defined || len(alias.TypeParams) > 0 || alias.Target == nil ||
				alias.Target.Form != symbol.FormNamed || len(alias.Target.Args) > 0 {

				continue
			}
			if _, held := sets[alias.Name]; !held {
				sets[alias.Name] = valueSet{alias: alias, test: test}
			}
		}
	}
	if len(sets) == 0 {
		return nil, nil
	}

	variants := map[*node.Alias][]*node.EnumVariant{}
	moved := map[symbol.Symbol]symbol.Symbol{}
	for _, file := range files {
		test := strings.HasSuffix(file.Path, golang.TestSuffix+golang.Extension)
		for _, decl := range file.Decls {
			c, is := decl.(*node.Constant)
			if !is || c.Type == nil || len(c.Type.Args) > 0 {
				continue
			}
			set, matches := sets[c.Type.Spelling]
			if !matches || test && !set.test {
				continue
			}
			variant := &node.EnumVariant{
				Pos: c.Pos, Doc: c.Doc, Name: c.Name,
				Value:       c.Value,
				Comment:     c.Comment,
				Annotations: c.Annotations,
			}
			variants[set.alias] = append(variants[set.alias], variant)
			moved[c] = variant
		}
	}
	if len(variants) == 0 {
		return nil, nil
	}

	promoted := make(map[*node.Alias]*node.Enum, len(variants))
	for _, file := range files {
		kept := file.Decls[:0]
		for _, decl := range file.Decls {
			if _, consumed := moved[decl]; consumed {
				continue
			}
			alias, is := decl.(*node.Alias)
			if !is || len(variants[alias]) == 0 {
				kept = append(kept, decl)
				continue
			}
			enum := &node.Enum{
				Pos: alias.Pos, Doc: alias.Doc, Name: alias.Name,
				Comment:     alias.Comment,
				Visibility:  alias.Visibility,
				Underlying:  alias.Target,
				Variants:    variants[alias],
				Annotations: alias.Annotations,
			}
			promoted[alias] = enum
			moved[alias] = enum
			kept = append(kept, enum)
		}
		file.Decls = kept
	}
	return promoted, moved
}

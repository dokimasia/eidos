// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// promoteEnums rewrites the idiomatic value sets of one file into the
// Enum kind. A value set is a defined type over an ordered basic type
// whose constants all have that type. It becomes one Enum:
//
//   - the constants become its variants in declaration order, with the
//     spellings of their values verbatim, iota arithmetic included;
//   - the target of the defined type, such as uint8, becomes its
//     underlying type;
//   - the methods of the type in the file fold into it.
//
// A type without constants, a constant group of another type and a type
// whose constants are in another file stay as parsed, because promotion
// reads one file. [foldMethods] folds the methods of an enumeration from
// the other files of the package afterwards.
//
// The rewrite runs after a file lowers, over the declarations of that
// file alone. It returns two maps of what it replaced. The first maps
// each alias to its enum, and the deferred underlying stamps read it. The
// second maps each consumed declaration to its replacement, and the
// caller re-homes the recorded attachments and stamps through it. A
// folded method keeps its own node and needs no re-homing.
func promoteEnums(
	file *node.File,
) (map[*node.Alias]*node.Enum, map[symbol.Symbol]symbol.Symbol) {
	aliases := map[string]*node.Alias{}
	for _, decl := range file.Decls {
		if alias, is := decl.(*node.Alias); is && alias.Defined && basicUnderlying(alias) {
			aliases[alias.Name] = alias
		}
	}
	if len(aliases) == 0 {
		return nil, nil
	}

	variants := map[string][]*node.EnumVariant{}
	consumed := map[symbol.Symbol]bool{}
	moved := map[symbol.Symbol]symbol.Symbol{}
	for _, decl := range file.Decls {
		c, is := decl.(*node.Constant)
		if !is || c.Type == nil || len(c.Type.Args) > 0 {
			continue
		}
		alias, matches := aliases[c.Type.Spelling]
		if !matches {
			continue
		}
		variant := &node.EnumVariant{
			Pos: c.Pos, Doc: c.Doc, Name: c.Name,
			Value:       c.Value,
			Comment:     c.Comment,
			Annotations: c.Annotations,
		}
		variants[alias.Name] = append(variants[alias.Name], variant)
		consumed[c] = true
		moved[c] = variant
	}
	if len(consumed) == 0 {
		return nil, nil
	}

	methods := map[string][]*node.Method{}
	for _, decl := range file.Decls {
		m, is := decl.(*node.Method)
		if !is || m.Receives == nil {
			continue
		}
		if _, promotes := variants[m.Receives.Spelling]; promotes {
			methods[m.Receives.Spelling] = append(methods[m.Receives.Spelling], m)
			consumed[m] = true
		}
	}

	promoted := map[*node.Alias]*node.Enum{}
	out := make(node.Symbols, 0, len(file.Decls))
	for _, decl := range file.Decls {
		if consumed[decl] {
			continue
		}
		alias, is := decl.(*node.Alias)
		if !is || len(variants[alias.Name]) == 0 {
			out = append(out, decl)
			continue
		}
		enum := &node.Enum{
			Pos: alias.Pos, Doc: alias.Doc, Name: alias.Name,
			Comment:     alias.Comment,
			Visibility:  alias.Visibility,
			Underlying:  alias.Target,
			Variants:    variants[alias.Name],
			Methods:     methods[alias.Name],
			Annotations: alias.Annotations,
		}
		promoted[alias] = enum
		moved[alias] = enum
		out = append(out, enum)
	}
	file.Decls = out
	return promoted, moved
}

// basicUnderlying reports whether the target of a defined type is an
// ordered basic type, which [golang.Ordered] decides: an integer, a float
// or a string. An idiomatic value set has such a type.
func basicUnderlying(alias *node.Alias) bool {
	return alias.Target != nil && len(alias.Target.Args) == 0 && golang.Ordered(alias.Target.Spelling)
}

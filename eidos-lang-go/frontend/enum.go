// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package frontend

import (
	golang "go.dokimi.dev/eidos/lang/go"
	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// promoteEnums rewrites one file's idiomatic value sets into the
// Enum kind the schema names for them: a defined type over an
// ordered basic type whose constants all name it as their type
// collapses into one Enum, the constants its variants in
// declaration order, the type's methods in the file folded in, and
// the value spellings kept verbatim, iota arithmetic included. A
// type with no constants, a constant group typed by anything else,
// and a type whose constants are declared in another file are kept
// as parsed: promotion reads one file, and [foldMethods] folds the
// enumeration's methods from the package's other files afterwards.
//
// The rewrite runs after a file lowers, over its own declarations
// alone, and returns what it replaced two ways: the alias→enum map
// the deferred underlying stamps read, and the full
// consumed→replacement map the caller re-homes recorded attachments
// and stamps through, so authored intent follows the declaration
// that replaces the consumed one. Folded methods keep their own
// nodes and need no re-homing.
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

// basicUnderlying reports whether a defined type's target is an
// ordered basic type, [golang.Ordered]: an integer, a float or a
// string, the types an idiomatic value set is declared over.
func basicUnderlying(alias *node.Alias) bool {
	return alias.Target != nil && len(alias.Target.Args) == 0 && golang.Ordered(alias.Target.Spelling)
}

// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package rules

import (
	"strings"

	"go.dokimi.dev/eidos/sdk/node"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The attributes that the rules read: the derive attribute, and the
// attribute that marks the default variant of an enum.
const (
	deriveAttr  = "derive"
	defaultAttr = "default"
)

// The traits of the standard library that the capabilities read, which a
// derive attribute generates or a trait impl names.
const (
	defaultTrait   = "Default"
	partialEqTrait = "PartialEq"
	eqTrait        = "Eq"
	partialOrd     = "PartialOrd"
	ordTrait       = "Ord"
)

// stdName returns the last segment of the spelling of a named reference ref
// without a target whose package is the prelude or a path of the standard
// library, as Debug for std::fmt::Debug, and reports false for every other
// reference. It allocates nothing.
func stdName(ref *node.TypeRef) (string, bool) {
	if ref == nil || ref.Form != symbol.FormNamed || !ref.Target.IsZero() {
		return "", false
	}
	if head, _, _ := strings.Cut(ref.Package, packageSep); ref.Package != "" && head != stdCrate &&
		head != coreCrate && head != allocCrate {

		return "", false
	}
	name := ref.Spelling
	if _, last, qualified := strings.CutLast(name, pathSep); qualified {
		name = last
	}
	return name, true
}

// derives reports whether a derive attribute among the annotations names the
// trait, by the last segment of its path, as #[derive(Debug, PartialEq)]
// names PartialEq. It allocates nothing.
func derives(annotations symbol.Annotations, trait string) bool {
	for _, an := range annotations {
		if an.Name != deriveAttr {
			continue
		}
		for _, arg := range an.Args {
			arg = strings.TrimSpace(arg)
			if _, last, qualified := strings.CutLast(arg, pathSep); qualified {
				arg = last
			}
			if arg == trait {
				return true
			}
		}
	}
	return false
}

// implementsTrait reports whether a reference among refs, such as the
// implements clause of a struct or the bounds of a type parameter, names the
// trait of the standard library without type arguments. A trait with type
// arguments, as PartialEq<Other>, is another trait, so it does not count. It
// allocates nothing.
func implementsTrait(refs []*node.TypeRef, trait string) bool {
	for _, ref := range refs {
		if name, std := stdName(ref); std && name == trait && len(ref.Args) == 0 {
			return true
		}
	}
	return false
}

// hasTrait reports whether the declaration sym has the trait of the standard
// library: a struct that derives it or that a trait impl implements it for,
// and an enum or a data enum that derives it. The frontend adds the trait
// impls of a struct alone to its implements clause, so a trait impl for an
// enum does not count. Every other declaration has no trait. It allocates
// nothing.
func hasTrait(sym symbol.Symbol, trait string) bool {
	var (
		annotations symbol.Annotations
		implements  []*node.TypeRef
	)
	switch d := sym.(type) {
	case *node.Struct:
		if d != nil {
			annotations, implements = d.Annotations, d.Implements
		}
	case *node.Enum:
		if d != nil {
			annotations = d.Annotations
		}
	case *node.Sum:
		if d != nil {
			annotations = d.Annotations
		}
	}
	return derives(annotations, trait) || implementsTrait(implements, trait)
}

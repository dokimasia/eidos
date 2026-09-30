// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spellref

import (
	"fmt"
	"strings"

	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// Spell writes one reference: the spelling, and any arguments
// recursively inside the given brackets. A nil reference or an
// empty spelling writes the stand-in, which each language names
// for itself — an anonymous marker, or its unit type.
func Spell(t *emit.TypeRef, opener, closer, standIn string) string {
	if t == nil || t.Spelling == "" {
		return standIn
	}
	if len(t.Args) == 0 {
		return t.Spelling
	}
	args := make([]string, 0, len(t.Args))
	for _, a := range t.Args {
		args = append(args, Spell(a, opener, closer, standIn))
	}
	return t.Spelling + opener + strings.Join(args, ", ") + closer
}

// Qualify spells one named reference for a language: the name the
// file refers to it by, binding the import that name needs.
type Qualify func(t *emit.TypeRef) (string, error)

// SpellWith writes one reference the way [Spell] does, and spells
// every named reference through qualify: the reference itself, each
// argument and each child of a structural reference. A structural
// reference keeps its written spelling where every named reference
// inside it spells as written, and SpellWith returns an error where
// one spells otherwise, because in the languages that call it a
// composite's spelling does not follow from its structure: a list is
// an array or a collection type, a map a record or a class. An error
// qualify returns is returned as it is.
func SpellWith(t *emit.TypeRef, opener, closer, standIn string, qualify Qualify) (string, error) {
	out, _, err := spellWith(t, opener, closer, standIn, qualify)
	return out, err
}

// spellWith is [SpellWith], reporting whether the result is the
// reference's written spelling.
func spellWith(
	t *emit.TypeRef, opener, closer, standIn string, qualify Qualify,
) (string, bool, error) {
	if t == nil || t.Spelling == "" {
		return standIn, true, nil
	}
	if t.Form != symbol.FormNamed {
		for _, c := range t.Elems {
			if _, written, err := spellWith(c, opener, closer, standIn, qualify); err != nil || !written {
				if err == nil {
					err = fmt.Errorf("%s names a type the file imports under another name, "+
						"and its spelling does not follow from its structure", t.Spelling)
				}
				return "", false, err
			}
		}
		return t.Spelling, true, nil
	}
	name, err := qualify(t)
	if err != nil {
		return "", false, err
	}
	written := name == t.Spelling
	if len(t.Args) == 0 {
		return name, written, nil
	}
	args := make([]string, 0, len(t.Args))
	for _, a := range t.Args {
		arg, argWritten, err := spellWith(a, opener, closer, standIn, qualify)
		if err != nil {
			return "", false, err
		}
		written = written && argWritten
		args = append(args, arg)
	}
	return name + opener + strings.Join(args, ", ") + closer, written, nil
}

// PackageOf returns the package a named reference names for a backend
// of one language: its target's package where the target is a
// declaration of that language, nothing where the target is another
// language's, whose package no import of this language names, and
// the package the reference records where it has no target.
func PackageOf(t *emit.TypeRef, lang symbol.Lang) string {
	switch {
	case t.Target.IsZero():
		return t.Package
	case t.Target.Lang == lang:
		return t.Target.Package
	default:
		return ""
	}
}

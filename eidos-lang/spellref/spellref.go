// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spellref

import (
	"fmt"
	"strings"

	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// argSeparator separates the arguments a spelling writes inside its
// brackets.
const argSeparator = ", "

// Spell writes one reference: the spelling, and any arguments
// recursively inside the given brackets. A nil reference or an
// empty spelling writes the stand-in, which each language names
// for itself: an anonymous marker, or its unit type.
//
// # Allocation contract
//
// A reference without arguments returns its spelling and allocates
// nothing. A reference with arguments writes into one buffer sized to
// the result, one allocation.
func Spell(t *emit.TypeRef, opener, closer, standIn string) string {
	if t == nil || t.Spelling == "" {
		return standIn
	}
	if len(t.Args) == 0 {
		return t.Spelling
	}
	var b strings.Builder
	b.Grow(spelledLen(t, opener, closer, standIn))
	writeSpelled(&b, t, opener, closer, standIn)
	return b.String()
}

// spelledLen returns the length of what [Spell] writes for t.
func spelledLen(t *emit.TypeRef, opener, closer, standIn string) int {
	if t == nil || t.Spelling == "" {
		return len(standIn)
	}
	if len(t.Args) == 0 {
		return len(t.Spelling)
	}
	n := len(t.Spelling) + len(opener) + len(closer) + len(argSeparator)*(len(t.Args)-1)
	for _, a := range t.Args {
		n += spelledLen(a, opener, closer, standIn)
	}
	return n
}

// writeSpelled writes what [Spell] returns for t into b.
func writeSpelled(b *strings.Builder, t *emit.TypeRef, opener, closer, standIn string) {
	if t == nil || t.Spelling == "" {
		b.WriteString(standIn)
		return
	}
	b.WriteString(t.Spelling)
	if len(t.Args) == 0 {
		return
	}
	b.WriteString(opener)
	for i, a := range t.Args {
		if i > 0 {
			b.WriteString(argSeparator)
		}
		writeSpelled(b, a, opener, closer, standIn)
	}
	b.WriteString(closer)
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
//
// # Allocation contract
//
// A structural reference, and a named one without arguments, return a
// spelling without allocating beyond what qualify allocates. A named
// reference with arguments writes into one buffer, one allocation where
// the qualified names fit the written spellings' length.
func SpellWith(t *emit.TypeRef, opener, closer, standIn string, qualify Qualify) (string, error) {
	if t == nil || t.Spelling == "" {
		return standIn, nil
	}
	if t.Form != symbol.FormNamed {
		if _, err := spellWith(nil, t, opener, closer, standIn, qualify); err != nil {
			return "", err
		}
		return t.Spelling, nil
	}
	if len(t.Args) == 0 {
		return qualify(t)
	}
	var b strings.Builder
	b.Grow(spelledLen(t, opener, closer, standIn))
	if _, err := spellWith(&b, t, opener, closer, standIn, qualify); err != nil {
		return "", err
	}
	return b.String(), nil
}

// spellWith writes what [SpellWith] returns for t into b, and writes
// nothing where b is nil. It reports whether t spells as written: every
// named reference inside it qualified to its own spelling.
func spellWith(
	b *strings.Builder, t *emit.TypeRef, opener, closer, standIn string, qualify Qualify,
) (bool, error) {
	if t == nil || t.Spelling == "" {
		write(b, standIn)
		return true, nil
	}
	if t.Form != symbol.FormNamed {
		for _, c := range t.Elems {
			written, err := spellWith(nil, c, opener, closer, standIn, qualify)
			if err != nil {
				return false, err
			}
			if !written {
				return false, fmt.Errorf("%s names a type the file imports under another name, "+
					"and its spelling does not follow from its structure", t.Spelling)
			}
		}
		write(b, t.Spelling)
		return true, nil
	}
	name, err := qualify(t)
	if err != nil {
		return false, err
	}
	written := name == t.Spelling
	write(b, name)
	if len(t.Args) == 0 {
		return written, nil
	}
	write(b, opener)
	for i, a := range t.Args {
		if i > 0 {
			write(b, argSeparator)
		}
		argWritten, err := spellWith(b, a, opener, closer, standIn, qualify)
		if err != nil {
			return false, err
		}
		written = written && argWritten
	}
	write(b, closer)
	return written, nil
}

// write writes s into b, and nothing where b is nil.
func write(b *strings.Builder, s string) {
	if b != nil {
		b.WriteString(s)
	}
}

// PackageOf returns the package of a named reference for a backend of
// the language lang. It returns the target's package where the target is
// a declaration of lang, and the package that the reference records
// otherwise. A reference without a target records the package of its
// source. A translated reference has a target in another language, and
// the layout records the package of the file that declares its referent.
// PackageOf allocates nothing.
func PackageOf(t *emit.TypeRef, lang symbol.Lang) string {
	if !t.Target.IsZero() && t.Target.Lang == lang {
		return t.Target.Package
	}
	return t.Package
}

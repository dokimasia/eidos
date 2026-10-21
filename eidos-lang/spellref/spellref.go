// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package spellref

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// argSeparator separates the arguments a spelling writes inside its
// brackets.
const argSeparator = ", "

// spellingsStack is the number of spellings of children that
// [SpellRestated] keeps on the stack for one structural reference. A
// reference with more children allocates their list.
const spellingsStack = 4

// errUnspelled is the error of [SpellWith] for a structural reference
// without a spelling. rules.Substitute leaves such a reference where it
// replaced a type parameter inside it.
var errUnspelled = errors.New("spellref: a structural type without a written spelling has no spelling " +
	"that follows from its structure")

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
// an array or a collection type, a map a record or a class. For the same
// reason, SpellWith returns an error for a structural reference without
// a spelling, at any depth. An error qualify returns is returned as it
// is.
//
// # Allocation contract
//
// A structural reference, and a named one without arguments, return a
// spelling without allocating beyond what qualify allocates. A named
// reference with arguments writes into one buffer, one allocation where
// the qualified names fit the written spellings' length.
func SpellWith(t *emit.TypeRef, opener, closer, standIn string, qualify Qualify) (string, error) {
	if t == nil {
		return standIn, nil
	}
	if t.Form != symbol.FormNamed {
		if _, err := spellWith(nil, t, opener, closer, standIn, qualify); err != nil {
			return "", err
		}
		return t.Spelling, nil
	}
	if t.Spelling == "" {
		return standIn, nil
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
	if t == nil || (t.Spelling == "" && t.Form == symbol.FormNamed) {
		write(b, standIn)
		return true, nil
	}
	if t.Form != symbol.FormNamed {
		if t.Spelling == "" {
			return false, errUnspelled
		}
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

// Restate writes a structural reference from the spellings of its
// children, for a language whose composite spelling follows from its
// structure. elems has the spelling of each child of t, in the order of
// t.Elems.
type Restate func(t *emit.TypeRef, elems []string) (string, error)

// SpellRestated writes one reference as [SpellWith] does, and restates a
// structural reference through restate where a child spells otherwise
// than written or where the reference has no spelling. A structural
// reference with a spelling whose children spell as written keeps its
// written spelling, so a function type keeps the names of its
// parameters. An argument and a child spell by the same rules, so a
// restated child is part of the spelling of its parent. An error of
// qualify or of restate is returned as it is.
//
// # Allocation contract
//
// A structural reference whose children spell as written, and a named
// reference without arguments, return a spelling without allocating
// beyond what qualify allocates. A named reference with arguments writes
// into one buffer, one allocation where the qualified names fit the
// written spellings' length. A restated reference allocates the list of
// the spellings of its children, and what restate allocates. A structural
// reference with more than four children allocates their list once more.
func SpellRestated(
	t *emit.TypeRef, opener, closer, standIn string, qualify Qualify, restate Restate,
) (string, error) {
	out, _, err := spellRestated(t, opener, closer, standIn, qualify, restate)
	return out, err
}

// spellRestated returns what [SpellRestated] returns for t, and reports
// whether it is the written spelling of t: every named reference inside t
// qualified to its own spelling. It passes each parameter on by itself,
// so qualify and restate do not escape.
func spellRestated(
	t *emit.TypeRef, opener, closer, standIn string, qualify Qualify, restate Restate,
) (string, bool, error) {
	switch {
	case t == nil:
		return standIn, true, nil
	case t.Form != symbol.FormNamed:
		return restatedStructural(t, opener, closer, standIn, qualify, restate)
	case t.Spelling == "":
		return standIn, true, nil
	case len(t.Args) == 0:
		name, err := qualify(t)
		if err != nil {
			return "", false, err
		}
		return name, name == t.Spelling, nil
	}
	var b strings.Builder
	b.Grow(spelledLen(t, opener, closer, standIn))
	written, err := writeRestated(&b, t, opener, closer, standIn, qualify, restate)
	if err != nil {
		return "", false, err
	}
	return b.String(), written, nil
}

// writeRestated writes the spelling of t into b, and reports whether it is
// the written spelling of t. A named reference writes its arguments into
// b in turn, and every other reference writes what [spellRestated]
// returns.
func writeRestated(
	b *strings.Builder, t *emit.TypeRef, opener, closer, standIn string, qualify Qualify, restate Restate,
) (bool, error) {
	if t == nil || t.Spelling == "" || t.Form != symbol.FormNamed {
		out, written, err := spellRestated(t, opener, closer, standIn, qualify, restate)
		if err != nil {
			return false, err
		}
		b.WriteString(out)
		return written, nil
	}
	name, err := qualify(t)
	if err != nil {
		return false, err
	}
	written := name == t.Spelling
	b.WriteString(name)
	if len(t.Args) == 0 {
		return written, nil
	}
	b.WriteString(opener)
	for i, a := range t.Args {
		if i > 0 {
			b.WriteString(argSeparator)
		}
		argWritten, err := writeRestated(b, a, opener, closer, standIn, qualify, restate)
		if err != nil {
			return false, err
		}
		written = written && argWritten
	}
	b.WriteString(closer)
	return written, nil
}

// restatedStructural returns the written spelling of the structural
// reference t where t has one and each child spells as written, and the
// restatement of t from the spellings of its children otherwise.
func restatedStructural(
	t *emit.TypeRef, opener, closer, standIn string, qualify Qualify, restate Restate,
) (string, bool, error) {
	var stack [spellingsStack]string
	elems := stack[:0]
	written := true
	for _, c := range t.Elems {
		out, childWritten, err := spellRestated(c, opener, closer, standIn, qualify, restate)
		if err != nil {
			return "", false, err
		}
		written = written && childWritten
		elems = append(elems, out)
	}
	if written && t.Spelling != "" {
		return t.Spelling, true, nil
	}
	out, err := restate(t, slices.Clone(elems))
	return out, false, err
}

// TypeArgs writes a list of type parameters or of type references as the
// type arguments of a reference: the entries between opener and closer,
// separated by commas, and nothing for an empty list or nil. A type
// parameter writes its name, so a generic declaration refers to itself, as
// in *Builder[T]. A type reference writes what spell returns for it, so
// the file imports the package of each argument, as in the witnesses
// [int] of a test. TypeArgs returns an error for a list of another type,
// and the error of spell as it is.
//
// # Allocation contract
//
// TypeArgs allocates the text that it writes in one buffer of its size,
// and the list of its entries where the list has more than two, beside
// what spell allocates. An empty list allocates nothing.
func TypeArgs(list any, opener, closer string, spell func(*emit.TypeRef) (string, error)) (string, error) {
	var parts []string
	switch l := list.(type) {
	case nil:
		return "", nil
	case []*emit.TypeParam:
		parts = make([]string, 0, len(l))
		for _, tp := range l {
			parts = append(parts, tp.Name)
		}
	case []*emit.TypeRef:
		parts = make([]string, 0, len(l))
		for _, t := range l {
			spelled, err := spell(t)
			if err != nil {
				return "", err
			}
			parts = append(parts, spelled)
		}
	default:
		return "", fmt.Errorf("spellref: type arguments are a list of type parameters or of type references, "+
			"and the list is a %T", list)
	}
	if len(parts) == 0 {
		return "", nil
	}
	n := len(opener) + len(closer) + len(argSeparator)*(len(parts)-1)
	for _, p := range parts {
		n += len(p)
	}
	var b strings.Builder
	b.Grow(n)
	b.WriteString(opener)
	for i, p := range parts {
		if i > 0 {
			b.WriteString(argSeparator)
		}
		b.WriteString(p)
	}
	b.WriteString(closer)
	return b.String(), nil
}

// PackageOf returns the package of a named reference for a backend of
// the language lang. It returns the target's package where the target is
// a declaration of lang and the reference is not emitted. It returns the
// package that the reference records otherwise. A reference without a
// target records the package of its source. A translated reference has a
// target in another language, and the layout records the package of the
// file that declares its referent. The layout records the same package
// for an emitted reference. Such a reference has [emit.TypeRef.Emitted]
// set, and its target can be in any language. A reference to a type
// parameter has no package, because no package exports a type parameter.
// PackageOf returns the empty package for it, so the backend writes it
// without an import. PackageOf allocates nothing.
func PackageOf(t *emit.TypeRef, lang symbol.Lang) string {
	if t.Target.Kind == symbol.KindTypeParam {
		return ""
	}
	if !t.Emitted && t.Target.Lang == lang {
		return t.Target.Package
	}
	return t.Package
}

// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package typescript

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The punctuation of the composite types that [Restate] writes.
const (
	listMark          = "[]"
	readonlyWord      = "readonly "
	tupleOpen         = "["
	tupleClose        = "]"
	groupOpen         = "("
	groupClose        = ")"
	unionSep          = " | "
	intersectionSep   = " & "
	elemSep           = ", "
	indexOpen         = "{ [key: "
	readonlyIndexOpen = "{ readonly [key: "
	indexSep          = "]: "
	indexClose        = " }"
	paramName         = "p"
	paramSep          = ": "
	funcArrow         = ") => "
	voidType          = "void"
	// restateSlack is the length that Restate adds to the spellings of the
	// children for the punctuation of one composite, beyond which the
	// spelling grows once more.
	restateSlack = 32
	// paramScratch is the stack buffer that the index of a parameter is
	// written into.
	paramScratch = 4
)

// The refusals of [Restate] for a function type.
var (
	errRestList = errors.New("typescript: a restated function type takes a list last, " +
		"and the reference does not record whether it is a rest parameter")
	errResults = errors.New("typescript: a restated function type has more than one result, " +
		"and TypeScript returns one")
)

// Restate writes a structural reference of TypeScript from the spellings
// of its children. elems has the spelling of each child of t, in the
// order of t.Elems. Restate writes a list as T[], a tuple as [A, B] and a
// map as an object type of one index signature, each with readonly where
// t is immutable. It writes a union, an intersection and a function type,
// whose parameters are p0, p1 and so on. A child is in parentheses where
// [Grouped] reports true for it. Restate is the restatement of TypeScript
// for [spellref.SpellRestated].
//
// Error modes:
//   - A reference with fewer spellings than its form takes returns an
//     error.
//   - An optional returns an error, because an emitted optional does not
//     record which absent value its source wrote.
//   - A function type whose last parameter is a list returns an error,
//     because the reference does not record whether the parameter is a
//     rest parameter. A function type with more than one result returns
//     an error.
//   - An array, a stream, a borrow, a wildcard, an inline type and every
//     other form return an error, because no TypeScript spelling of them
//     follows from their children.
//
// # Allocation contract
//
// Restate allocates the spelling that it returns, and an error allocates
// its message.
func Restate(t *emit.TypeRef, elems []string) (string, error) {
	want := 0
	switch t.Form {
	case symbol.FormList, symbol.FormUnion, symbol.FormIntersection:
		want = 1
	case symbol.FormMap:
		want = 2
	case symbol.FormFunc:
		want = t.Split
	default:
		// A tuple takes any count of children, and every other form
		// returns its error below.
	}
	if len(elems) != len(t.Elems) || len(elems) < want || t.Split > len(elems) {
		return "", fmt.Errorf("typescript: a restated type has %d spellings for %d children, "+
			"and its form takes %d", len(elems), len(t.Elems), want)
	}
	size := restateSlack
	for _, e := range elems {
		size += len(e) + len(elemSep) + len(groupOpen) + len(groupClose)
	}
	var b strings.Builder
	b.Grow(size)
	switch t.Form {
	case symbol.FormList:
		if t.Mutability == symbol.MutabilityImmutable {
			b.WriteString(readonlyWord)
		}
		writeChild(&b, t.Form, t.Elems[0], elems[0])
		b.WriteString(listMark)
	case symbol.FormTuple:
		if t.Mutability == symbol.MutabilityImmutable {
			b.WriteString(readonlyWord)
		}
		b.WriteString(tupleOpen)
		writeJoined(&b, t, elems, elemSep)
		b.WriteString(tupleClose)
	case symbol.FormMap:
		if t.Mutability == symbol.MutabilityImmutable {
			b.WriteString(readonlyIndexOpen)
		} else {
			b.WriteString(indexOpen)
		}
		b.WriteString(elems[0])
		b.WriteString(indexSep)
		b.WriteString(elems[1])
		b.WriteString(indexClose)
	case symbol.FormUnion:
		writeJoined(&b, t, elems, unionSep)
	case symbol.FormIntersection:
		writeJoined(&b, t, elems, intersectionSep)
	case symbol.FormFunc:
		if err := writeFunc(&b, t, elems); err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("typescript: a restated type has a form without a TypeScript spelling "+
			"that follows from its children: %s", strings.Join(elems, elemSep))
	}
	return b.String(), nil
}

// writeFunc writes a function type of t into b: each parameter as p0,
// p1 and so on with its type, then the one result, or void.
//
// Error modes: a last parameter that is a list and more than one result
// return an error.
func writeFunc(b *strings.Builder, t *emit.TypeRef, elems []string) error {
	if t.Split > 0 && t.Elems[t.Split-1] != nil && t.Elems[t.Split-1].Form == symbol.FormList {
		return errRestList
	}
	results := elems[t.Split:]
	if len(results) > 1 {
		return errResults
	}
	var scratch [paramScratch]byte
	b.WriteString(groupOpen)
	for i, p := range elems[:t.Split] {
		if i > 0 {
			b.WriteString(elemSep)
		}
		b.WriteString(paramName)
		b.Write(strconv.AppendInt(scratch[:0], int64(i), 10))
		b.WriteString(paramSep)
		b.WriteString(p)
	}
	b.WriteString(funcArrow)
	if len(results) == 0 {
		b.WriteString(voidType)
		return nil
	}
	b.WriteString(results[0])
	return nil
}

// writeJoined writes the children of t into b with sep between them, each
// in parentheses where the form of t binds it otherwise.
func writeJoined(b *strings.Builder, t *emit.TypeRef, elems []string, sep string) {
	for i, e := range elems[:len(t.Elems)] {
		if i > 0 {
			b.WriteString(sep)
		}
		writeChild(b, t.Form, t.Elems[i], e)
	}
}

// writeChild writes the spelling text of the type child into b. It puts
// the text in parentheses where [Grouped] reports true for outer and
// child.
func writeChild(b *strings.Builder, outer symbol.TypeForm, child *emit.TypeRef, text string) {
	if !Grouped(outer, child) {
		b.WriteString(text)
		return
	}
	b.WriteString(groupOpen)
	b.WriteString(text)
	b.WriteString(groupClose)
}

// Grouped reports whether the type child needs parentheses inside a
// composite of the form outer. It reports true for these children:
//
//   - a union, an intersection, a function type and a read-only
//     collection inside a list;
//   - a function type inside a union;
//   - a union or a function type inside an intersection.
//
// TypeScript parses each of them differently without the parentheses. A
// tuple, a map and a function type separate their children with commas
// and colons, so Grouped reports false for their children. It also
// reports false for a nil child. Grouped allocates nothing.
func Grouped(outer symbol.TypeForm, child *emit.TypeRef) bool {
	if child == nil {
		return false
	}
	switch outer {
	case symbol.FormList:
		return child.Form == symbol.FormUnion || child.Form == symbol.FormIntersection ||
			child.Form == symbol.FormFunc ||
			(child.Form == symbol.FormList || child.Form == symbol.FormTuple) &&
				child.Mutability == symbol.MutabilityImmutable
	case symbol.FormUnion:
		return child.Form == symbol.FormFunc
	case symbol.FormIntersection:
		return child.Form == symbol.FormUnion || child.Form == symbol.FormFunc
	default:
		return false
	}
}

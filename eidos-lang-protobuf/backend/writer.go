// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"errors"
	"fmt"
	"strconv"

	"go.dokimi.dev/eidos/lang/protobuf"
	"go.dokimi.dev/eidos/lang/spoke"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/rules"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The syntax of the composite forms of a field type. A list writes its
// label before its element, a stream writes its mark before its message,
// and a map writes its key and its value in angle brackets.
const (
	repeatedLabel = "repeated "
	streamMark    = "stream "
	mapOpen       = "map<"
	mapSep        = ", "
	mapClose      = ">"
)

// The phrases that [arity] writes for the number of children that a form
// takes.
const (
	oneChild    = "one child"
	twoChildren = "two children"
)

// errNoType is the refusal of a declaration without a type.
var errNoType = errors.New(refusalPrefix + "the declaration states no type")

// Writer writes the references of one protobuf file: the type of a field,
// the request and the response of an rpc, and the message that an extend
// block extends. It records the import of the file that each reference
// names into the import set of the file, so the import block lists every
// file that the file refers to. A named reference is written by these
// rules:
//
//   - A reference into the file itself, and a reference without a
//     package, is written as it is and imports nothing.
//   - A reference to a well-known type is written as it is, and imports
//     the file that declares the type.
//   - A reference to a declaration of protobuf is written as its source
//     writes it, and imports the file that its package names.
//   - Any other reference into a file of the plan imports that file. It is
//     qualified with the package clause of that file, where the file has a
//     clause and the clause differs from the clause of the file under
//     render. [render.ImportSet.Referent] returns the package of the file.
//   - A reference into a file outside the plan is written as it is, and
//     imports the file.
//
// # Concurrency
//
// A Writer is not safe for concurrent use, because its set is not.
//
// # Allocation contract
//
// A reference written as it is allocates nothing. A qualified reference
// allocates its qualified name, and a composite form allocates its
// spelling. The first import of a file grows the set. A refusal allocates
// its error.
type Writer struct {
	set *render.ImportSet
}

// NewWriter returns the writer of the file whose import set is set. set
// must not be nil. NewWriter allocates nothing.
func NewWriter(set *render.ImportSet) Writer { return Writer{set: set} }

// Spell writes the reference t in the position of the type of a field. A
// named reference spells by the rules of [Writer]. A list spells as its
// element under the label repeated, a map as map<K, V>, a stream as its
// message under the mark stream, and an optional as its type, because the
// field writes its presence.
//
// Error modes, each a refusal of the declaration that states t:
//   - no reference, and a named reference without a spelling;
//   - a reference with type arguments, which protobuf has none of;
//   - a list, an optional, a stream or a map of a composite form, which
//     protobuf writes through a wrapper message;
//   - a form with the wrong number of children;
//   - every other form, which protobuf does not spell.
func (w Writer) Spell(t *emit.TypeRef) (string, error) {
	if t == nil {
		return "", errNoType
	}
	switch t.Form {
	case symbol.FormNamed:
		return w.named(t)
	case symbol.FormOptional, symbol.FormList, symbol.FormStream:
		if err := arity(t, 1); err != nil {
			return "", err
		}
		elem, err := w.element(t, t.Elems[0])
		switch {
		case err != nil:
			return "", err
		case t.Form == symbol.FormList:
			return repeatedLabel + elem, nil
		case t.Form == symbol.FormStream:
			return streamMark + elem, nil
		default:
			return elem, nil
		}
	case symbol.FormMap:
		if err := arity(t, 2); err != nil {
			return "", err
		}
		key, err := w.element(t, t.Elems[0])
		if err != nil {
			return "", err
		}
		value, err := w.element(t, t.Elems[1])
		if err != nil {
			return "", err
		}
		return mapOpen + key + mapSep + value + mapClose, nil
	default:
		return "", fmt.Errorf(refusalPrefix+"%s is %s, which protobuf does not spell",
			t.Spelling, spoke.Describe(rules.TypeShape{Form: t.Form, Async: t.Async}))
	}
}

// element writes the child c of the composite reference t. A child of a
// composite form is a named reference, because protobuf writes a field of
// a composite of a composite through a wrapper message.
func (w Writer) element(t, c *emit.TypeRef) (string, error) {
	if c.Form != symbol.FormNamed {
		return "", fmt.Errorf(
			refusalPrefix+"%s is %s of %s, and protobuf writes such a field through a wrapper message",
			t.Spelling,
			spoke.Describe(rules.TypeShape{Form: t.Form, Async: t.Async}),
			spoke.Describe(rules.TypeShape{Form: c.Form, Async: c.Async}),
		)
	}
	return w.named(c)
}

// named writes the named reference t by the rules of [Writer], and records
// the import of the file that it names.
func (w Writer) named(t *emit.TypeRef) (string, error) {
	switch {
	case t.Spelling == "":
		return "", errNoType
	case len(t.Args) > 0:
		return "", fmt.Errorf(refusalPrefix+"%s states %d type arguments, and protobuf has none",
			t.Spelling, len(t.Args))
	}
	file := t.Package
	if known, wellKnown := protobuf.WellKnownImport(t.Spelling); file == "" && wellKnown {
		file = known
	}
	if file == "" || file == w.set.Home() {
		return t.Spelling, nil
	}
	w.set.Add(file)
	if t.Target.Lang == protobuf.Lang {
		return t.Spelling, nil
	}
	referent, held := w.set.Referent(file)
	if !held || referent.Name == "" || referent.Name == w.set.Package().Name {
		return t.Spelling, nil
	}
	return referent.Name + protobuf.NameSep + t.Spelling, nil
}

// arity returns an error where the reference t does not have n children,
// in the words of [spoke.Arity].
func arity(t *emit.TypeRef, n int) error {
	if len(t.Elems) == n {
		return nil
	}
	want := oneChild
	if n == 2 {
		want = twoChildren
	}
	return errors.New(refusalPrefix + spoke.Describe(rules.TypeShape{Form: t.Form, Async: t.Async}) +
		" takes " + want + ", and the shape has " + strconv.Itoa(len(t.Elems)))
}

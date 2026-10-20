// Copyright Dokimasia B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"strconv"
	"strings"

	"go.dokimi.dev/eidos/lang/python"
	"go.dokimi.dev/eidos/lang/scaffold"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The truth values of Python, and the texts that the value tree states for
// them.
const (
	trueLiteral  = "True"
	falseLiteral = "False"
	trueText     = "true"
	falseText    = "false"
)

// The parts of the literals of the collections of Python and of a call:
// the brackets of a tuple, the comma of a tuple of one element, the colon of
// an entry of a dict and the equals sign of a keyword argument, and the
// names of the collections that a composite writes as a literal.
const (
	tupleOpen    = "("
	tupleClose   = ")"
	tupleSingle  = ",)"
	entrySep     = ": "
	keywordEqual = "="
	setName      = "set"
	frozenName   = "frozenset"
	sequenceName = "Sequence"
)

// The parts of a lambda: its keyword, the colon before its body, and the
// start of the name of a parameter that the body does not read, which the
// position of the parameter ends. A name that starts with an underscore is
// the name of an unused variable for the linters of Python. paramDigits is
// the size of the stack buffer that the position is written into.
const (
	lambdaKeyword = "lambda"
	lambdaColon   = ": "
	lambdaParam   = "_"
	paramDigits   = 4
)

// target spells a value tree as Python into the import set of a file. Each
// spelling of a reference imports what it names through a writer of the
// set, which reads the declaration whose body the set names. A target
// allocates nothing of its own.
type target struct{ set *render.ImportSet }

// Lang names the target in a refusal.
func (target) Lang() string { return string(python.Lang) }

// Literal spells one leaf of a value: a number as its text, a string as
// [python.Quote] writes it, a truth value as True or False, the absent value
// as None, and raw text of Python as it is.
//
// Error modes, each a refusal of the value: a number without text, a truth
// value that is neither true nor false, raw text of another language, and a
// literal kind that the vocabulary does not declare.
func (target) Literal(v emit.Value) (string, error) {
	lang := string(python.Lang)
	switch v.Literal {
	case emit.LiteralInt, emit.LiteralFloat:
		if v.Text == "" {
			return "", render.RefuseValue(lang, "the %s literal has no text", v.Literal)
		}
		return v.Text, nil
	case emit.LiteralString:
		return python.Quote(v.Text), nil
	case emit.LiteralBool:
		switch v.Text {
		case trueText:
			return trueLiteral, nil
		case falseText:
			return falseLiteral, nil
		default:
			return "", render.RefuseValue(lang, "a boolean literal is true or false, not %q", v.Text)
		}
	case emit.LiteralNil:
		return noneName, nil
	case emit.LiteralRaw:
		if v.Lang != python.Lang {
			return "", render.RefuseValue(lang, "%q is written in %s, not %s", v.Text, v.Lang, lang)
		}
		return v.Text, nil
	default:
		return "", render.RefuseValue(lang, "no spelling for the %s literal", v.Literal)
	}
}

// Type spells the reference of a value, as a call of the type writes it.
func (t target) Type(ref *emit.TypeRef) (string, error) {
	w := Writer{set: t.set}
	return w.spellLazy(ref)
}

// Callee spells the declaration that a call or a name value refers to: the
// name of a declaration of the file, and the name that the import of a
// declaration of another module binds. A member of a type is the member
// after its type, as Color.RED for a member of an enum, and the type is
// what the file imports.
func (t target) Callee(id symbol.Identity) (string, error) {
	head, rest := id.Name, ""
	if id.Owner != "" {
		owner, nested, _ := strings.Cut(id.Owner, symbol.OwnerSep)
		head = owner
		rest = id.Name
		if nested != "" {
			rest = nested + dot + id.Name
		}
	}
	local := head
	if id.Package != "" && id.Package != t.set.Home() {
		local = t.set.BindItem(id.Package, head, false)
	}
	if rest == "" {
		return local, nil
	}
	return local + dot + rest, nil
}

// Conversion spells a conversion as a call of its type, as UserId(0) for a
// NewType.
func (target) Conversion(_ *emit.TypeRef, typ, inner string) (string, error) {
	return typ + callOpen + inner + callClose, nil
}

// Composite spells a composite of a type as the value that its annotation
// admits: a list literal for a list and the sequences of builtins and of
// collections.abc, a tuple literal for a tuple, an immutable list and an
// array, which an annotation writes as tuples, a set literal for a set, a
// frozenset of a set literal for a frozenset, and a dict literal for a map
// and a mapping. Any other type is a call of the type, with a positional
// argument for each element and a keyword argument for each named field,
// as a dataclass and a TypedDict take them.
//
// Error modes: an entry that the literal or the call cannot write, such as
// a named field of a list or a key of a call, and a positional argument of
// a call after a keyword argument.
func (target) Composite(ref *emit.TypeRef, typ string, entries []scaffold.Entry) (string, error) {
	kind := compositeOf(ref)
	switch kind {
	case listComposite:
		return elements(argsOpen, entries, argsClose, typ)
	case tupleComposite:
		if len(entries) == 1 && entries[0].Name == "" && entries[0].Key == "" {
			return tupleOpen + entries[0].Value + tupleSingle, nil
		}
		return elements(tupleOpen, entries, tupleClose, typ)
	case setComposite, frozenComposite:
		if len(entries) == 0 {
			return typ + callOpen + callClose, nil
		}
		set, err := elements(dictOpen, entries, dictClose, typ)
		if err != nil || kind == setComposite {
			return set, err
		}
		return typ + callOpen + set + callClose, nil
	case mapComposite:
		parts := make([]string, 0, len(entries))
		for _, e := range entries {
			if e.Key == "" {
				return "", render.RefuseValue(string(python.Lang), "a %s value has an entry without a key", typ)
			}
			parts = append(parts, e.Key+entrySep+e.Value)
		}
		return dictOpen + strings.Join(parts, argSep) + dictClose, nil
	default:
		parts := make([]string, 0, len(entries))
		named := false
		for _, e := range entries {
			switch {
			case e.Key != "":
				return "", render.RefuseValue(string(python.Lang), "a call of %s has no keyed argument", typ)
			case e.Name != "":
				named = true
				parts = append(parts, e.Name+keywordEqual+e.Value)
			case named:
				return "", render.RefuseValue(string(python.Lang),
					"a call of %s takes its positional arguments before its keyword arguments", typ)
			default:
				parts = append(parts, e.Value)
			}
		}
		return typ + callOpen + strings.Join(parts, argSep) + callClose, nil
	}
}

// Address refuses, because Python has no address of a value.
func (target) Address(emit.Value, string) (string, error) {
	return "", render.RefuseValue(string(python.Lang), "Python spells no address of a value")
}

// Construction spells a construction as a call of its type with its
// arguments, as datetime(2023, 11, 14).
func (t target) Construction(ref *emit.TypeRef, args []string) (string, error) {
	w := Writer{set: t.set}
	typ, err := w.spellLazy(ref)
	if err != nil {
		return "", err
	}
	return typ + callOpen + strings.Join(args, argSep) + callClose, nil
}

// Func spells a lambda of a function type, as lambda _0, _1: True. It
// writes one parameter for each parameter of the type, because a lambda
// with fewer parameters is not a value of the Callable. The body is the
// result. It is None for a function without results, and a tuple for a
// function of two results or more, because the Callable of such a function
// returns a tuple. Func writes no type, so the file imports nothing for the
// lambda. A type that is not a function type refuses, such as
// Callable[..., R], whose reference does not state its parameters.
//
// Func allocates the spelling, one allocation. A position of five digits
// or more allocates its text too.
func (t target) Func(ref *emit.TypeRef, results []string) (string, error) {
	if ref.Form != symbol.FormFunc {
		return "", render.RefuseValue(t.Lang(),
			"Python writes a lambda of a function type that states its parameters, and %s states none", ref.Spelling)
	}
	params := max(ref.Split, 0)
	n := len(lambdaKeyword) + len(" ") + params*(len(lambdaParam)+paramDigits+len(argSep)) + len(lambdaColon) +
		len(tupleOpen) + len(tupleClose) + len(noneName)
	for _, r := range results {
		n += len(r) + len(argSep)
	}
	var b strings.Builder
	b.Grow(n)
	b.WriteString(lambdaKeyword)
	var scratch [paramDigits]byte
	for i := range params {
		if i == 0 {
			b.WriteByte(' ')
		} else {
			b.WriteString(argSep)
		}
		b.WriteString(lambdaParam)
		b.Write(strconv.AppendInt(scratch[:0], int64(i), 10))
	}
	b.WriteString(lambdaColon)
	switch len(results) {
	case 0:
		b.WriteString(noneName)
	case 1:
		b.WriteString(results[0])
	default:
		b.WriteString(tupleOpen)
		for i, r := range results {
			if i > 0 {
				b.WriteString(argSep)
			}
			b.WriteString(r)
		}
		b.WriteString(tupleClose)
	}
	return b.String(), nil
}

// Make refuses: Python has no make.
func (t target) Make(_ *emit.TypeRef, typ string) (string, error) {
	return "", render.RefuseValue(t.Lang(), "Python has no make of %s", typ)
}

// Value spells one value of the value tree as Python, through a target that
// imports into set what the references and the callees of the value name.
// The value builtin calls it, so a template writes a value as a body of the
// same file writes it. A value that Python has no form for returns a
// [render.ValueError].
//
// Value allocates what [scaffold.Value] allocates and the text of each part
// of the value that the target joins. A number, a truth value and None
// allocate nothing.
func Value(v emit.Value, set *render.ImportSet) (string, error) {
	return scaffold.Value(target{set: set}, v)
}

// composite is the literal that a composite of a type writes.
type composite uint8

const (
	callComposite composite = iota
	listComposite
	tupleComposite
	setComposite
	frozenComposite
	mapComposite
)

// compositeOf returns the literal of a composite of the type ref: by the
// form of a structural reference, and by the name of a builtin or of a
// collection of collections.abc for a named one. The walk of a value passes
// a composite with its type alone.
func compositeOf(ref *emit.TypeRef) composite {
	switch ref.Form {
	case symbol.FormList:
		if ref.Mutability == symbol.MutabilityImmutable {
			return tupleComposite
		}
		return listComposite
	case symbol.FormArray, symbol.FormTuple:
		return tupleComposite
	case symbol.FormMap:
		return mapComposite
	case symbol.FormNamed:
	default:
		return callComposite
	}
	switch ref.Spelling {
	case listName, sequenceName:
		return listComposite
	case tupleName:
		return tupleComposite
	case setName:
		return setComposite
	case frozenName:
		return frozenComposite
	case dictName, mappingName:
		return mapComposite
	default:
		return callComposite
	}
}

// elements spells the elements of a literal between open and closer, and
// refuses a named field or a key, which typ does not take.
func elements(open string, entries []scaffold.Entry, closer, typ string) (string, error) {
	parts := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.Name != "" || e.Key != "" {
			return "", render.RefuseValue(string(python.Lang), "a %s value has a named entry, and its literal "+
				"takes elements alone", typ)
		}
		parts = append(parts, e.Value)
	}
	return open + strings.Join(parts, argSep) + closer, nil
}

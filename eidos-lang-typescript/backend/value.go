// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"strconv"
	"strings"

	"go.dokimi.dev/eidos/lang/scaffold"
	typescript "go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// The spellings TypeScript's value forms are written with.
const (
	// nullSpelling is TypeScript's absent value. The vocabulary
	// carries one absent value and TypeScript has two, so the
	// derived one is null, which is what a JSON-shaped value takes.
	nullSpelling = "null"
	// trueSpelling and falseSpelling are the two truth values.
	trueSpelling  = "true"
	falseSpelling = "false"
)

// target spells a value tree as TypeScript, recording into the
// file's import set whatever its references and callees need.
type target struct{ set *render.ImportSet }

// Lang names the target for a refusal.
func (target) Lang() string { return string(typescript.Lang) }

// Literal spells one leaf. A number carries its own text; a string
// quotes through the standard library, whose escapes TypeScript
// reads the same way; a boolean takes exactly the two spellings;
// the absent value is null. Raw text spells only where the author
// wrote it in TypeScript.
func (t target) Literal(v emit.Value) (string, error) {
	switch v.Literal {
	case emit.LiteralInt, emit.LiteralFloat:
		if v.Text == "" {
			return "", render.RefuseValue(t.Lang(), "a %s literal carries no text", v.Literal)
		}
		return v.Text, nil
	case emit.LiteralString:
		return strconv.Quote(v.Text), nil
	case emit.LiteralBool:
		if v.Text != trueSpelling && v.Text != falseSpelling {
			return "", render.RefuseValue(t.Lang(),
				"a boolean literal spells %s or %s, not %q", trueSpelling, falseSpelling, v.Text)
		}
		return v.Text, nil
	case emit.LiteralNil:
		return nullSpelling, nil
	case emit.LiteralRaw:
		if v.Lang != typescript.Lang {
			return "", render.RefuseValue(t.Lang(),
				"%q is written in %s, which TypeScript cannot spell", v.Text, v.Lang)
		}
		return v.Text, nil
	default:
		return "", render.RefuseValue(t.Lang(), "no spelling for the %s literal", v.Literal)
	}
}

// Type spells a reference and records the named binding its module
// needs.
func (t target) Type(ref *emit.TypeRef) (string, error) {
	if ref == nil || ref.Spelling == "" {
		return "", render.RefuseValue(t.Lang(), "a value names a type that spells nothing")
	}
	t.use(ref.Target)
	return Spell(ref), nil
}

// Callee spells a function by name and records its binding: an
// imported name resolves bare in TypeScript, so nothing qualifies.
func (t target) Callee(id symbol.Identity) (string, error) {
	if id.Name == "" {
		return "", render.RefuseValue(t.Lang(), "a call names a function that spells nothing")
	}
	t.use(id)
	return id.Name, nil
}

// Conversion spells a type assertion. TypeScript erases its types,
// so a defined type has no constructor to call and the value is
// its inner one, asserted to the type the projection named.
func (target) Conversion(_ *emit.TypeRef, typ, inner string) (string, error) {
	return inner + " as " + typ, nil
}

// Composite spells an object literal for a record and a map, and
// an array literal for a list: the reference's form decides, and a
// form TypeScript has no literal for refuses.
func (t target) Composite(
	ref *emit.TypeRef, typ string, entries []scaffold.Entry,
) (string, error) {
	if ref != nil && (ref.Form == symbol.FormList || ref.Form == symbol.FormArray) {
		parts := make([]string, 0, len(entries))
		for _, e := range entries {
			if e.Name != "" || e.Key != "" {
				return "", render.RefuseValue(t.Lang(),
					"a %s value carries a named entry, and an array literal takes none", typ)
			}
			parts = append(parts, e.Value)
		}
		return "[" + strings.Join(parts, ", ") + "]", nil
	}
	parts := make([]string, 0, len(entries))
	for _, e := range entries {
		switch {
		case e.Name != "":
			parts = append(parts, e.Name+": "+e.Value)
		case e.Key != "":
			parts = append(parts, "["+e.Key+"]: "+e.Value)
		default:
			return "", render.RefuseValue(t.Lang(),
				"a %s value carries a positional element, and an object literal names every entry", typ)
		}
	}
	return "{" + strings.Join(parts, ", ") + "}", nil
}

// Call spells an application.
func (target) Call(callee string, args []string) (string, error) {
	return callee + "(" + strings.Join(args, ", ") + ")", nil
}

// Address refuses: TypeScript has no address operator, and a
// reference to a value is the value.
func (t target) Address(string) (string, error) {
	return "", render.RefuseValue(t.Lang(), "TypeScript spells no address of a value")
}

// use records the binding a reference or a callee in another
// module needs.
func (t target) use(id symbol.Identity) {
	if id.Package == "" || id.Name == "" || t.set == nil {
		return
	}
	t.set.AddNamed(id.Package, id.Name)
}

// Copyright ThesmOS B.V. 2026
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"strings"

	"go.dokimi.dev/eidos/lang/scaffold"
	typescript "go.dokimi.dev/eidos/lang/typescript"
	"go.dokimi.dev/eidos/sdk/emit"
	"go.dokimi.dev/eidos/sdk/render"
	"go.dokimi.dev/eidos/sdk/symbol"
)

// leaves is TypeScript's spelling of the literal leaves. A number
// keeps its text, and a string quotes in TypeScript's own grammar.
// The vocabulary has one absent value and TypeScript has two, so the
// derived one is null, which a JSON-shaped value takes.
var leaves = scaffold.Leaves{
	Lang:   typescript.Lang,
	Absent: "null",
	Quote:  quote,
}

// target spells a value tree as TypeScript, recording into the
// file's import set the bindings its references and callees need.
type target struct{ set *render.ImportSet }

// Lang names the target for a refusal.
func (target) Lang() string { return string(typescript.Lang) }

// Literal spells one leaf through [leaves].
func (target) Literal(v emit.Value) (string, error) { return leaves.Literal(v) }

// Type spells a reference and records the named binding its module
// needs.
func (t target) Type(ref *emit.TypeRef) (string, error) {
	t.use(ref.Target)
	return Spell(ref), nil
}

// Callee spells a function by name and records its binding: an
// imported name resolves bare in TypeScript, so nothing qualifies.
func (t target) Callee(id symbol.Identity) (string, error) {
	t.use(id)
	return id.Name, nil
}

// Conversion spells a type assertion. TypeScript erases its types,
// so a defined type has no constructor to call, and the value is its
// inner one, asserted to the type the projection named.
func (target) Conversion(_ *emit.TypeRef, typ, inner string) (string, error) {
	return inner + " as " + typ, nil
}

// Composite spells an object literal for a record and a map, and an
// array literal for a list. The reference's form decides, and a form
// TypeScript has no literal for is refused.
func (t target) Composite(
	ref *emit.TypeRef, typ string, entries []scaffold.Entry,
) (string, error) {
	if ref != nil && (ref.Form == symbol.FormList || ref.Form == symbol.FormArray) {
		parts := make([]string, 0, len(entries))
		for _, e := range entries {
			if e.Name != "" || e.Key != "" {
				return "", render.RefuseValue(t.Lang(),
					"a %s value has a named entry, and an array literal takes none", typ)
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
				"a %s value has a positional element, and an object literal names every entry", typ)
		}
	}
	return "{" + strings.Join(parts, ", ") + "}", nil
}

// Address refuses: TypeScript has no address operator, and a
// reference to a value is the value.
func (t target) Address(emit.Value, string) (string, error) {
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
